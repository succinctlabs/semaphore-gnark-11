#!/bin/bash
set -e

# Script to create a public S3 bucket in us-west-1 for trusted setup files

REGION="us-west-1"

if [ -z "$1" ]; then
    echo "Usage: $0 <bucket-name>"
    echo "Example: $0 succinct-trusted-setup-test"
    exit 1
fi

BUCKET_NAME="$1"

echo "Creating S3 bucket: $BUCKET_NAME in region: $REGION"

# Create the bucket
aws s3api create-bucket \
    --bucket "$BUCKET_NAME" \
    --region "$REGION" \
    --create-bucket-configuration LocationConstraint="$REGION"

echo "Disabling block public access..."

# Disable block public access
aws s3api put-public-access-block \
    --bucket "$BUCKET_NAME" \
    --public-access-block-configuration \
    "BlockPublicAcls=false,IgnorePublicAcls=false,BlockPublicPolicy=false,RestrictPublicBuckets=false"

echo "Applying bucket policy for public read access..."

# Create and apply bucket policy
POLICY=$(cat <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "Stmt1405592139000",
            "Effect": "Allow",
            "Principal": "*",
            "Action": [
                "s3:GetObject",
                "s3:ListBucket"
            ],
            "Resource": [
                "arn:aws:s3:::${BUCKET_NAME}/*",
                "arn:aws:s3:::${BUCKET_NAME}"
            ]
        }
    ]
}
EOF
)

aws s3api put-bucket-policy \
    --bucket "$BUCKET_NAME" \
    --policy "$POLICY"

echo "Successfully created public S3 bucket: $BUCKET_NAME"
echo "Bucket URL: https://${BUCKET_NAME}.s3.${REGION}.amazonaws.com"
