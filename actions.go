package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"time"
	"unsafe"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	groth16 "github.com/consensys/gnark/backend/groth16/bn254"
	"github.com/consensys/gnark/backend/groth16/bn254/mpcsetup"
	"github.com/consensys/gnark/backend/solidity"
	cs "github.com/consensys/gnark/constraint/bn254"
	"github.com/urfave/cli/v2"
	deserializer "github.com/worldcoin/ptau-deserializer/deserialize"
)

const Region = "us-west-1"

// getPhase1Commons extracts the unexported SrsCommons from Phase1 using reflection
func getPhase1Commons(phase1 *mpcsetup.Phase1) *mpcsetup.SrsCommons {
	v := reflect.ValueOf(phase1).Elem()
	params := v.FieldByName("parameters")
	return (*mpcsetup.SrsCommons)(unsafe.Pointer(params.UnsafeAddr()))
}

// computePhase2Hash computes the hash of a Phase2 by serializing and hashing
func computePhase2Hash(phase2 *mpcsetup.Phase2) []byte {
	var buf bytes.Buffer
	phase2.WriteTo(&buf)
	h := sha256.Sum256(buf.Bytes())
	return h[:]
}

func p1i(cCtx *cli.Context) error {
	// sanity check
	if cCtx.Args().Len() != 2 {
		return errors.New("please provide the correct arguments")
	}

	ptauFilePath := cCtx.Args().Get(0)
	outputFilePath := cCtx.Args().Get(1)

	ptau, err := deserializer.ReadPtau(ptauFilePath)
	if err != nil {
		return err
	}

	phase1, err := deserializer.ConvertPtauToPhase1(ptau)
	if err != nil {
		return err
	}

	outputFile, err := os.Create(outputFilePath)
	if err != nil {
		return err
	}

	_, err = phase1.WriteTo(outputFile)
	if err != nil {
		return err
	}

	return nil
}

func p2n(cCtx *cli.Context) error {
	if cCtx.Args().Len() != 4 {
		return errors.New("please provide the correct arguments")
	}

	phase1Path := cCtx.Args().Get(0)
	r1csPath := cCtx.Args().Get(1)
	phase2Path := cCtx.Args().Get(2)
	evalsPath := cCtx.Args().Get(3)

	fmt.Println("reading phase1")
	phase1File, err := os.Open(phase1Path)
	if err != nil {
		return err
	}
	defer phase1File.Close()
	phase1 := &mpcsetup.Phase1{}
	phase1.ReadFrom(phase1File)

	fmt.Println("reading r1cs")
	r1csFile, err := os.Open(r1csPath)
	if err != nil {
		return err
	}
	defer r1csFile.Close()
	r1cs := &cs.R1CS{}
	r1cs.ReadFrom(r1csFile)

	// Get SrsCommons from Phase1 using reflection
	commons := getPhase1Commons(phase1)

	fmt.Println("initializing phase2")
	phase2 := &mpcsetup.Phase2{}
	evals := phase2.Initialize(r1cs, commons)

	fmt.Println("writing phase2")
	phase2File, err := os.Create(phase2Path)
	if err != nil {
		return err
	}
	defer phase2File.Close()
	phase2.WriteTo(phase2File)

	// Write evals - serialize manually since Phase2Evaluations doesn't have WriteTo in new API
	fmt.Println("writing evals")
	evalsFile, err := os.Create(evalsPath)
	if err != nil {
		return err
	}
	defer evalsFile.Close()
	if err := writePhase2Evaluations(&evals, evalsFile); err != nil {
		return err
	}

	return nil
}

func p2c(cCtx *cli.Context) error {
	var err error
	if cCtx.Args().Len() != 2 {
		return errors.New("please provide the correct arguments")
	}

	presignedUploadUrl := cCtx.Args().Get(0)
	bucketName := cCtx.Args().Get(1)

	re := regexp.MustCompile(`phase2(?:-(?<index>\d+))?\?`)
	matches := re.FindStringSubmatch(presignedUploadUrl)
	contributionIndex := 0

	if matches == nil {
		return fmt.Errorf("The presigned URL doesn't contain a phase2 object key")
	}

	if matches[1] != "" {
		contributionIndex, err = strconv.Atoi(matches[1])
		if err != nil {
			return err
		}
	}

	previousContributionObjectKey := "phase2"

	if contributionIndex > 0 {
		previousContributionObjectKey = fmt.Sprintf("%s-%d", previousContributionObjectKey, contributionIndex-1)
	}

	outputPh2Path := fmt.Sprintf("./trusted-setup/phase2-%d", contributionIndex)

	svc, err := GetS3Service(Region, true)
	if err != nil {
		return err
	}

	fmt.Printf("Downloading previous contribution: %s\n", previousContributionObjectKey)
	inputPh2Path, err := Download(svc, previousContributionObjectKey, bucketName)
	if err != nil {
		return err
	}

	inputFile, err := os.Open(*inputPh2Path)
	if err != nil {
		return err
	}
	defer inputFile.Close()
	phase2 := &mpcsetup.Phase2{}
	phase2.ReadFrom(inputFile)

	fmt.Printf("Generating contribution\n")
	phase2.Contribute()

	outputFile, err := os.Create(outputPh2Path)
	if err != nil {
		return err
	}
	defer outputFile.Close()
	phase2.WriteTo(outputFile)

	fmt.Printf("Uploading contribution: phase2-%d\n", contributionIndex)
	err = Upload(outputPh2Path, presignedUploadUrl)
	if err != nil {
		return err
	}

	// Compute hash of the contribution
	hash := hex.EncodeToString(computePhase2Hash(phase2))

	downloadURL := fmt.Sprintf("http://%s.s3.%s.amazonaws.com/phase2-%d",
		bucketName,
		Region,
		contributionIndex,
	)

	fmt.Printf("Contribution successful!\n")
	fmt.Printf("Once your contribution has been verified by the coordinator, you can attest for it on social media, providing the following info:\n")
	fmt.Printf(" - Contribution URL: %s\n", downloadURL)
	fmt.Printf(" - Contribution Hash: %s\n", hash)

	return nil
}

func p2v(cCtx *cli.Context) error {
	if cCtx.Args().Len() != 2 {
		return errors.New("please provide the correct arguments")
	}
	contributionIndex := cCtx.Args().Get(0)
	bucketName := cCtx.Args().Get(1)

	svc, err := GetS3Service(Region, true)
	if err != nil {
		return err
	}

	currentContribution := fmt.Sprintf("phase2-%s", contributionIndex)

	fmt.Printf("Downloading current contribution: %s\n", currentContribution)
	inputPath, err := Download(svc, currentContribution, bucketName)
	if err != nil {
		return err
	}

	inputFile, err := os.Open(*inputPath)
	if err != nil {
		return err
	}
	defer inputFile.Close()
	input := &mpcsetup.Phase2{}
	input.ReadFrom(inputFile)

	originKey := "phase2"
	idx, err := strconv.Atoi(contributionIndex)
	if err != nil {
		return err
	}
	if idx > 0 {
		originKey = fmt.Sprintf("phase2-%d", idx-1)
	}

	fmt.Printf("Downloading origin: %s\n", originKey)
	originPath, err := Download(svc, originKey, bucketName)
	if err != nil {
		return err
	}

	originFile, err := os.Open(*originPath)
	if err != nil {
		return err
	}
	defer originFile.Close()
	origin := &mpcsetup.Phase2{}
	origin.ReadFrom(originFile)

	fmt.Printf("Verifying contribution with hash: %s\n", hex.EncodeToString(computePhase2Hash(input)))
	if err := origin.Verify(input); err != nil {
		fmt.Println("Phase2 Verification failed:", err.Error())
		return err
	}

	fmt.Printf("Ok!\n")
	return nil
}

func p2u(cCtx *cli.Context) error {
	if cCtx.Args().Len() != 1 {
		return errors.New("please provide the correct arguments")
	}
	bucketName := cCtx.Args().Get(0)

	svc, err := GetS3Service(Region, false)
	if err != nil {
		return err
	}

	// Open the file
	file, err := os.Open("./trusted-setup/phase2")
	if err != nil {
		return err
	}
	defer file.Close()

	// Get file size and read the file content
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}

	// Create an S3 upload input parameters
	uploadInput := &s3.PutObjectInput{
		Bucket:        aws.String(bucketName),
		Key:           aws.String(filepath.Base("phase2")),
		Body:          file,
		ContentLength: aws.Int64(fileInfo.Size()),
	}

	// Upload the file
	_, err = svc.PutObject(uploadInput)
	if err != nil {
		return err
	}

	return nil
}

func presigned(cCtx *cli.Context) error {
	if cCtx.Args().Len() != 2 {
		return errors.New("please provide the correct arguments")
	}

	bucketName := cCtx.Args().Get(0)
	countStr := cCtx.Args().Get(1)

	count, err := strconv.Atoi(countStr)
	if err != nil {
		return err
	}

	putLifetime := 7 * 24 * time.Hour

	svc, err := GetS3Service(Region, false)
	if err != nil {
		return err
	}

	for i := 0; i < count; i++ {
		// Create the PutObjectInput parameters
		putObjectInput := &s3.PutObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(fmt.Sprintf("phase2-%d", i)),
		}

		// Create a request object for PutObject
		req, _ := svc.PutObjectRequest(putObjectInput)

		// Presign the request with the specified expiration
		putURLStr, err := req.Presign(putLifetime) // Use the PUT lifetime
		if err != nil {
			return err
		}

		fmt.Printf("%d: %s\n", i, putURLStr)

	}

	return nil
}

func keys(cCtx *cli.Context) error {
	// sanity check
	if cCtx.Args().Len() != 4 {
		return errors.New("please provide the correct arguments")
	}

	fmt.Println("reading phase1")
	phase1Path := cCtx.Args().Get(0)
	phase1 := &mpcsetup.Phase1{}
	phase1File, err := os.Open(phase1Path)
	if err != nil {
		return err
	}
	defer phase1File.Close()
	phase1.ReadFrom(phase1File)

	fmt.Println("reading phase2")
	phase2Path := cCtx.Args().Get(1)
	phase2 := &mpcsetup.Phase2{}
	phase2File, err := os.Open(phase2Path)
	if err != nil {
		return err
	}
	defer phase2File.Close()
	phase2.ReadFrom(phase2File)

	fmt.Println("reading evals")
	evalsPath := cCtx.Args().Get(2)
	evalsFile, err := os.Open(evalsPath)
	if err != nil {
		return err
	}
	defer evalsFile.Close()
	evals, err := readPhase2Evaluations(evalsFile)
	if err != nil {
		return err
	}

	fmt.Println("reading r1cs")
	r1csPath := cCtx.Args().Get(3)
	r1cs := &cs.R1CS{}
	r1csFile, err := os.Open(r1csPath)
	if err != nil {
		return err
	}
	defer r1csFile.Close()
	r1cs.ReadFrom(r1csFile)

	// Get SrsCommons from Phase1 using reflection
	commons := getPhase1Commons(phase1)

	// Use a deterministic beacon challenge for key extraction
	// In production, this should be a proper random beacon value
	beaconChallenge := []byte("semaphore-gnark-ceremony-beacon")

	fmt.Println("extracting keys")
	pk, vk := phase2.Seal(commons, evals, beaconChallenge)

	pkTyped := pk.(*groth16.ProvingKey)
	vkTyped := vk.(*groth16.VerifyingKey)

	fmt.Println("pk written")
	fmt.Println("cardinality", pkTyped.Domain.Cardinality)

	pkFile, err := os.Create("pk")
	if err != nil {
		return err
	}
	_, err = pk.WriteTo(pkFile)
	if err != nil {
		return err
	}

	vkFile, err := os.Create("vk")
	if err != nil {
		return err
	}
	defer vkFile.Close()
	vkTyped.WriteTo(vkFile)

	return nil
}

func sol(cCtx *cli.Context) error {
	// sanity check
	if cCtx.Args().Len() != 1 {
		return errors.New("please provide the correct arguments")
	}

	vkPath := cCtx.Args().Get(0)
	vk := &groth16.VerifyingKey{}
	vkFile, err := os.Open(vkPath)
	if err != nil {
		return err
	}
	defer vkFile.Close()
	vk.ReadFrom(vkFile)

	solFile, err := os.Create("Groth16Verifier.sol")
	if err != nil {
		return err
	}
	defer solFile.Close()

	err = vk.ExportSolidity(solFile, solidity.WithPragmaVersion("0.8.20"))
	return err
}

func GetS3Service(region string, anonymous bool) (*s3.S3, error) {
	// Create custom AWS configuration
	config := &aws.Config{
		Region: aws.String(region),
	}

	if anonymous {
		config.Credentials = credentials.AnonymousCredentials
	}

	customEndpoint, exists := os.LookupEnv("CUSTOM_ENDPOINT")

	if exists {
		config.Endpoint = aws.String(customEndpoint)
		config.S3ForcePathStyle = aws.Bool(true)
	}

	// Create a new AWS session with the custom config
	sess, err := session.NewSession(config)
	if err != nil {
		return nil, err
	}

	// Create S3 service client
	svc := s3.New(sess)

	return svc, nil
}

func Download(svc *s3.S3, objectKey string, bucketName string) (*string, error) {

	filePath := "./trusted-setup/" + objectKey

	// Create a new file for writing the S3 object contents to
	file, err := os.Create(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Create the download input parameters
	downloadInput := &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	}

	// Download the file from S3
	result, err := svc.GetObject(downloadInput)
	if err != nil {
		return nil, err
	}
	defer result.Body.Close()

	// Write the contents to the local file
	_, err = io.Copy(file, result.Body)
	if err != nil {
		return nil, err
	}

	return &filePath, nil
}

func Upload(filePath string, presignedURL string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}
	fileSize := fileInfo.Size()

	request, err := http.NewRequest(http.MethodPut, presignedURL, file)
	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/octet-stream")
	request.ContentLength = fileSize

	client := &http.Client{} // Use the default client or configure one if needed
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Upload failed: Status Code: %d", response.StatusCode)
	}

	return nil
}
