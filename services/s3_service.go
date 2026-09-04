package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Service struct{}

func (s S3Service) PutDiploma(ctx context.Context, uuidDocumento, pdfBase64 string) (S3ObjectRef, error) {
	bucket := diplomasS3Bucket()
	if bucket == "" {
		return S3ObjectRef{}, fmt.Errorf("%w: DiplomasS3Bucket is required", ErrInvalidInput)
	}
	if uuidDocumento == "" {
		return S3ObjectRef{}, fmt.Errorf("%w: uuid_documento is required", ErrInvalidInput)
	}

	pdf, err := base64.StdEncoding.DecodeString(pdfBase64)
	if err != nil {
		return S3ObjectRef{}, fmt.Errorf("%w: signed file is not valid base64", ErrInvalidInput)
	}

	options := []func(*config.LoadOptions) error{
		config.WithRegion(awsRegion()),
	}
	if awsAccessKeyID() != "" && awsSecretAccessKey() != "" {
		options = append(options, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(awsAccessKeyID(), awsSecretAccessKey(), ""),
		))
	}
	if awsEndpointURL() != "" {
		options = append(options, config.WithBaseEndpoint(awsEndpointURL()))
	}

	cfg, err := config.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return S3ObjectRef{}, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	key := fmt.Sprintf("diplomas/%s/diploma.pdf", uuidDocumento)
	output, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(pdf),
		ContentType: aws.String("application/pdf"),
	})
	if err != nil {
		return S3ObjectRef{}, fmt.Errorf("put diploma in s3: %w", err)
	}

	ref := S3ObjectRef{
		Bucket: bucket,
		Key:    key,
		URI:    fmt.Sprintf("s3://%s/%s", bucket, key),
	}
	if output.VersionId != nil {
		ref.VersionID = *output.VersionId
	}
	if output.ETag != nil {
		ref.ETag = *output.ETag
	}

	return ref, nil
}
