package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

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

const Region = "us-east-2"
const BucketName = "succinct-trusted-setup"

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

	phase1File, err := os.Open(phase1Path)
	if err != nil {
		return err
	}
	phase1 := &mpcsetup.Phase1{}
	phase1.ReadFrom(phase1File)

	r1csFile, err := os.Open(r1csPath)
	if err != nil {
		return err
	}
	r1cs := cs.R1CS{}
	r1cs.ReadFrom(r1csFile)

	phase2, evals := mpcsetup.InitPhase2(&r1cs, phase1)

	phase2File, err := os.Create(phase2Path)
	if err != nil {
		return err
	}
	phase2.WriteTo(phase2File)

	evalsFile, err := os.Create(evalsPath)
	if err != nil {
		return err
	}
	evals.WriteTo(evalsFile)

	return nil
}

func p2c(cCtx *cli.Context) error {
	var err error
	if cCtx.Args().Len() != 1 {
		return errors.New("please provide the correct arguments")
	}

	presignedUploadUrl := cCtx.Args().Get(0)
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
	inputPh2Path, err := Download(svc, previousContributionObjectKey)
	if err != nil {
		return err
	}

	inputFile, err := os.Open(*inputPh2Path)
	if err != nil {
		return err
	}
	phase2 := &mpcsetup.Phase2{}
	phase2.ReadFrom(inputFile)

	fmt.Printf("Generating contribution\n")
	phase2.Contribute()

	outputFile, err := os.Create(outputPh2Path)
	if err != nil {
		return err
	}
	phase2.WriteTo(outputFile)

	fmt.Printf("Uploading contribution: phase2-%d\n", contributionIndex)
	err = Upload(outputPh2Path, presignedUploadUrl)
	if err != nil {
		return err
	}

	return nil
}

func p2v(cCtx *cli.Context) error {
	if cCtx.Args().Len() != 1 {
		return errors.New("please provide the correct arguments")
	}
	contributionIndex := cCtx.Args().Get(0)

	svc, err := GetS3Service(Region, true)
	if err != nil {
		return err
	}

	currentContribution := fmt.Sprintf("phase2-%s", contributionIndex)

	fmt.Printf("Downloading current contribution: %s\n", currentContribution)
	inputPath, err := Download(svc, currentContribution)
	if err != nil {
		return err
	}

	inputFile, err := os.Open(*inputPath)
	if err != nil {
		return err
	}
	input := &mpcsetup.Phase2{}
	input.ReadFrom(inputFile)

	fmt.Printf("Downloading phase2\n")
	originPath, err := Download(svc, "phase2")
	if err != nil {
		return err
	}

	originFile, err := os.Open(*originPath)
	if err != nil {
		return err
	}
	origin := &mpcsetup.Phase2{}
	origin.ReadFrom(originFile)

	fmt.Printf("Verifying\n")
	mpcsetup.VerifyPhase2(origin, input)

	fmt.Printf("Ok!\n")
	return nil
}

func p2u(cCtx *cli.Context) error {

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
		Bucket:        aws.String(BucketName),
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
	if cCtx.Args().Len() != 1 {
		return errors.New("please provide the correct arguments")
	}

	countStr := cCtx.Args().Get(0)

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
			Bucket: aws.String(BucketName),
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
	phase1.ReadFrom(phase1File)

	fmt.Println("reading phase2")
	phase2Path := cCtx.Args().Get(1)
	phase2 := &mpcsetup.Phase2{}
	phase2File, err := os.Open(phase2Path)
	if err != nil {
		return err
	}
	phase2.ReadFrom(phase2File)

	fmt.Println("reading evals")
	evalsPath := cCtx.Args().Get(2)
	evals := &mpcsetup.Phase2Evaluations{}
	evalsFile, err := os.Open(evalsPath)
	if err != nil {
		return err
	}
	evals.ReadFrom(evalsFile)

	fmt.Println("reading r1cs")
	r1csPath := cCtx.Args().Get(3)
	r1cs := &cs.R1CS{}
	r1csFile, err := os.Open(r1csPath)
	if err != nil {
		return err
	}
	r1cs.ReadFrom(r1csFile)

	// get number of constraints
	nbConstraints := r1cs.GetNbConstraints()
	fmt.Println("extracting keys")
	pk, vk := mpcsetup.ExtractKeys(phase1, phase2, evals, nbConstraints)
	fmt.Println("pk written")
	fmt.Println("cardinality", pk.Domain.Cardinality)

	pkFile, err := os.Create("pk")
	if err != nil {
		return err
	}
	err = pk.WriteDump(pkFile)
	if err != nil {
		return err
	}

	vkFile, err := os.Create("vk")
	if err != nil {
		return err
	}
	vk.WriteTo(vkFile)

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
	vk.ReadFrom(vkFile)

	solFile, err := os.Create("Groth16Verifier.sol")
	if err != nil {
		return err
	}

	err = vk.ExportSolidity(solFile, solidity.WithPragmaVersion("0.8.20"))
	return err
}

func ClonePhase1(phase1 *mpcsetup.Phase1) mpcsetup.Phase1 {
	r := mpcsetup.Phase1{}
	r.Parameters.G1.Tau = append(r.Parameters.G1.Tau, phase1.Parameters.G1.Tau...)
	r.Parameters.G1.AlphaTau = append(r.Parameters.G1.AlphaTau, phase1.Parameters.G1.AlphaTau...)
	r.Parameters.G1.BetaTau = append(r.Parameters.G1.BetaTau, phase1.Parameters.G1.BetaTau...)

	r.Parameters.G2.Tau = append(r.Parameters.G2.Tau, phase1.Parameters.G2.Tau...)
	r.Parameters.G2.Beta = phase1.Parameters.G2.Beta

	r.PublicKeys = phase1.PublicKeys
	r.Hash = append(r.Hash, phase1.Hash...)

	return r
}

func ClonePhase2(phase2 *mpcsetup.Phase2) mpcsetup.Phase2 {
	r := mpcsetup.Phase2{}
	r.Parameters.G1.Delta = phase2.Parameters.G1.Delta
	r.Parameters.G1.L = append(r.Parameters.G1.L, phase2.Parameters.G1.L...)
	r.Parameters.G1.Z = append(r.Parameters.G1.Z, phase2.Parameters.G1.Z...)
	r.Parameters.G2.Delta = phase2.Parameters.G2.Delta
	r.PublicKey = phase2.PublicKey
	r.Hash = append(r.Hash, phase2.Hash...)

	return r
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

func Download(svc *s3.S3, objectKey string) (*string, error) {

	filePath := "./trusted-setup/" + objectKey

	// Create a new file for writing the S3 object contents to
	file, err := os.Create(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Create the download input parameters
	downloadInput := &s3.GetObjectInput{
		Bucket: aws.String(BucketName),
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
