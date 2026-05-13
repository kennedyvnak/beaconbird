package provider

import (
	"context"
	"fmt"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sestypes "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/kennedyvnak/beaconbird/internal/domain"
)

type sesClient interface {
	SendEmail(ctx context.Context, params *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
}

type SESProvider struct {
	client    sesClient
	fromEmail string
}

func NewSESProvider(ctx context.Context, region, accessKeyID, secretAccessKey, fromEmail string) (*SESProvider, error) {
	loadOptions := make([]func(*awsconfig.LoadOptions) error, 0, 2)
	if region == "" {
		return nil, fmt.Errorf("provider.NewSESProvider region is required")
	}
	loadOptions = append(loadOptions, awsconfig.WithRegion(region))

	switch {
	case accessKeyID == "" && secretAccessKey == "":
	case accessKeyID != "" && secretAccessKey != "":
		loadOptions = append(loadOptions, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")))
	default:
		return nil, fmt.Errorf("provider.NewSESProvider access key and secret key must be provided together")
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("provider.NewSESProvider config: %w", err)
	}
	return newSESProviderWithClient(sesv2.NewFromConfig(cfg), fromEmail)
}

func newSESProviderWithClient(client sesClient, fromEmail string) (*SESProvider, error) {
	if client == nil {
		return nil, fmt.Errorf("provider.newSESProviderWithClient client is required")
	}
	if fromEmail == "" {
		return nil, fmt.Errorf("provider.newSESProviderWithClient from email is required")
	}
	return &SESProvider{client: client, fromEmail: fromEmail}, nil
}

func (p *SESProvider) Channel() domain.Channel {
	return domain.ChannelEmail
}

func (p *SESProvider) Name() string {
	return "ses"
}

func (p *SESProvider) Send(ctx context.Context, job domain.DeliveryJob) (*ProviderResult, error) {
	toEmail, err := payloadString(job.Payload, "to_email")
	if err != nil {
		return nil, fmt.Errorf("ses.Send: %w", err)
	}
	subject, err := payloadString(job.Payload, "subject")
	if err != nil {
		return nil, fmt.Errorf("ses.Send: %w", err)
	}
	bodyText, hasText, err := payloadOptionalString(job.Payload, "body_text")
	if err != nil {
		return nil, fmt.Errorf("ses.Send: %w", err)
	}
	bodyHTML, hasHTML, err := payloadOptionalString(job.Payload, "body_html")
	if err != nil {
		return nil, fmt.Errorf("ses.Send: %w", err)
	}
	if !hasText && !hasHTML {
		return nil, fmt.Errorf("ses.Send: missing body_text or body_html")
	}

	input := &sesv2.SendEmailInput{
		FromEmailAddress: stringPtr(p.fromEmail),
		Destination: &sestypes.Destination{
			ToAddresses: []string{toEmail},
		},
		Content: &sestypes.EmailContent{
			Simple: &sestypes.Message{
				Subject: &sestypes.Content{Data: stringPtr(subject)},
				Body:    &sestypes.Body{},
			},
		},
	}
	if hasText {
		input.Content.Simple.Body.Text = &sestypes.Content{Data: stringPtr(bodyText)}
	}
	if hasHTML {
		input.Content.Simple.Body.Html = &sestypes.Content{Data: stringPtr(bodyHTML)}
	}

	output, err := p.client.SendEmail(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("ses.Send: %w", err)
	}

	result := &ProviderResult{}
	if output != nil && output.MessageId != nil {
		result.ProviderMessageID = *output.MessageId
	}
	return result, nil
}

func payloadOptionalString(payload map[string]any, key string) (string, bool, error) {
	value, ok := payload[key]
	if !ok || value == nil {
		return "", false, nil
	}
	text, ok := value.(string)
	if !ok {
		return "", false, fmt.Errorf("invalid %s", key)
	}
	return text, true, nil
}

func stringPtr(v string) *string {
	return &v
}
