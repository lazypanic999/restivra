package source

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/lazypanic999/restivra/internal/verify"
)

type Error struct{ Code, Message string }

func (e *Error) Error() string        { return e.Message }
func (e *Error) ErrorCode() string    { return e.Code }
func fail(code, message string) error { return &Error{code, message} }

// Never retain raw SDK diagnostics: they can contain request URLs, provider
// details or credential identifiers. Codes are stable and operator actionable.
func safeError(err error) error {
	if errors.Is(err, context.Canceled) {
		return fail("cancelled", "Source download was cancelled.")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fail("source_unavailable", "Source request exceeded its time limit.")
	}
	var response *smithyhttp.ResponseError
	if errors.As(err, &response) {
		switch response.HTTPStatusCode() {
		case http.StatusBadRequest, http.StatusNotImplemented:
			return fail("source_configuration", "S3 could not accept the request. Check the bucket, profile region and provider compatibility.")
		case http.StatusForbidden, http.StatusUnauthorized:
			return fail("source_access_denied", "S3 access denied. Check the worker profile and read-only bucket permissions.")
		case http.StatusNotFound:
			return fail("source_not_found", "The S3 bucket or selected object was not found.")
		case http.StatusPreconditionFailed:
			return fail("source_changed", "The selected S3 object changed before download. Use completed immutable backup objects.")
		case http.StatusMovedPermanently, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
			return fail("source_configuration", "S3 redirected the request. Check the profile endpoint and region.")
		}
	}
	var api smithy.APIError
	if errors.As(err, &api) && api.ErrorCode() == "AccessDenied" {
		return fail("source_access_denied", "S3 access denied. Check the worker profile and read-only bucket permissions.")
	}
	return fail("source_unavailable", "S3 is unavailable. Check the endpoint, network and worker source configuration.")
}

// A small interface enables conditional-request regression tests without
// replacing the SDK or the actual storage integration test.
type S3Client interface {
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	HeadObject(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

func Latest(ctx context.Context, client S3Client, bucket, pattern string) (types.Object, error) {
	prefix := pattern
	if i := strings.IndexAny(prefix, "*?["); i >= 0 {
		prefix = prefix[:i]
	}
	var selected types.Object
	var token *string
	count := 0
	pages := 0
	for {
		pages++
		if pages > 100 {
			return selected, fail("source_selection_limit", "S3 selection exceeds its bounded listing. Narrow the object pattern prefix.")
		}
		page, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: aws.String(bucket), Prefix: aws.String(prefix), MaxKeys: aws.Int32(1000), ContinuationToken: token})
		if err != nil {
			return selected, safeError(err)
		}
		for _, object := range page.Contents {
			count++
			if count > 10000 {
				return selected, fail("source_selection_limit", "S3 selection exceeds 10000 listed objects. Narrow the object pattern prefix.")
			}
			if object.Key == nil || object.LastModified == nil || strings.HasSuffix(*object.Key, "/") {
				continue
			}
			matches, _ := path.Match(pattern, *object.Key)
			if !matches {
				continue
			}
			if selected.Key == nil || object.LastModified.After(*selected.LastModified) || (object.LastModified.Equal(*selected.LastModified) && *object.Key > *selected.Key) {
				selected = object
			}
		}
		if !aws.ToBool(page.IsTruncated) {
			break
		}
		if count >= 10000 || page.NextContinuationToken == nil || aws.ToString(page.NextContinuationToken) == "" || aws.ToString(page.NextContinuationToken) == aws.ToString(token) {
			return selected, fail("source_selection_limit", "S3 selection exceeds its bounded listing. Narrow the object pattern prefix.")
		}
		token = page.NextContinuationToken
	}
	if selected.Key == nil {
		return selected, fail("source_not_found", "No completed S3 object matched the configured pattern.")
	}
	return selected, nil
}

func Snapshot(ctx context.Context, client S3Client, selection Selection, pattern, work string, max int64) (*os.File, verify.Backup, error) {
	b := verify.Backup{Source: &verify.BackupSource{Kind: "s3", Profile: selection.Profile, Bucket: selection.Bucket}}
	object, err := Latest(ctx, client, selection.Bucket, pattern)
	if err != nil {
		return nil, b, err
	}
	b.Name = path.Base(aws.ToString(object.Key))
	b.Source.Key = aws.ToString(object.Key)
	head, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(selection.Bucket), Key: object.Key})
	if err != nil {
		return nil, b, safeError(err)
	}
	if head.ContentLength == nil || head.LastModified == nil || head.ETag == nil || aws.ToString(head.ETag) == "" {
		return nil, b, fail("source_configuration", "S3 returned incomplete object metadata; safe conditional download is unavailable.")
	}
	b.Bytes = *head.ContentLength
	b.ModifiedAt = head.LastModified.UTC()
	b.Source.ETag = *head.ETag
	b.Source.VersionID = aws.ToString(head.VersionId)
	if b.Bytes < 0 || b.Bytes > max {
		return nil, b, fail("source_too_large", "S3 object exceeds the configured archive size limit.")
	}
	// Selection metadata must still describe the object tested. A replacement
	// between listing and HEAD is a failure rather than silently changing input.
	// List timestamps may carry milliseconds; HTTP Last-Modified on HEAD/GET
	// has whole-second precision. ETag, size and conditional GET bind the bytes.
	if !head.LastModified.Truncate(time.Second).Equal(object.LastModified.Truncate(time.Second)) || (object.ETag != nil && *object.ETag != *head.ETag) || (object.Size != nil && *object.Size != *head.ContentLength) {
		return nil, b, fail("source_changed", "The selected S3 object changed before download. Use completed immutable backup objects.")
	}
	input := &s3.GetObjectInput{Bucket: aws.String(selection.Bucket), Key: object.Key, IfMatch: head.ETag}
	if b.Source.VersionID != "" && b.Source.VersionID != "null" {
		input.VersionId = head.VersionId
	}
	result, err := client.GetObject(ctx, input)
	if err != nil {
		return nil, b, safeError(err)
	}
	defer result.Body.Close()
	if result.ETag == nil || *result.ETag != *head.ETag || result.ContentLength == nil || *result.ContentLength != b.Bytes || result.LastModified == nil || !result.LastModified.Equal(*head.LastModified) || (input.VersionId != nil && aws.ToString(result.VersionId) != *input.VersionId) {
		return nil, b, fail("source_changed", "S3 object metadata changed during download.")
	}
	snapshot, backup, err := verify.SnapshotStream(ctx, result.Body, b, work, max)
	if err != nil {
		if ctx.Err() != nil {
			return nil, backup, safeError(ctx.Err())
		}
		return nil, backup, fail("source_download_failed", "Could not create a complete PostgreSQL archive snapshot from S3. Check size, format and connection.")
	}
	return snapshot, backup, nil
}
