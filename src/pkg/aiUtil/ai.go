package aiUtil

import (
    "context"
    "errors"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/secret"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/sashabaranov/go-openai"
    "go.uber.org/zap"
)

type Ai struct {
    gptClient *openai.Client
    log       *zap.SugaredLogger
}

func NewAi(logger *zap.SugaredLogger) *Ai {
    return &Ai{
        gptClient: newGptClient(),
        log:       logger,
    }
}

func (ai *Ai) GenerateReply(review string) (string, error) {
    response, err := ai.gptClient.CreateChatCompletion(
        context.Background(),
        openai.ChatCompletionRequest{
            Temperature: 1.0,
            MaxTokens:   256,
            Model:       openai.GPT4,
            Messages: []openai.ChatCompletionMessage{
                {
                    Role:    openai.ChatMessageRoleSystem,
                    Content: util.AiReplyPrompt,
                },
                {
                    Role:    openai.ChatMessageRoleUser,
                    Content: review,
                },
            },
        },
    )
    if err != nil {
        e := &openai.APIError{}
        if errors.As(err, &e) {
            switch e.HTTPStatusCode {
            case 401:
                ai.log.Error("Error generating AI reply due to invalid API key: ", err)
            case 429:
                // rate limiting or engine overload (wait and retry)
                ai.log.Error("Error generating AI reply due to rate limit exceeded: ", err)
                // TODO: retry
            case 500:
                ai.log.Error("Error generating AI reply due to OpenAI server error: ", err)
                // TODO: retry
            default:
                ai.log.Error("Error generating AI reply due to unknown error: ", err)
            }
        }

        return "", err
    }

    if response.Choices[0].FinishReason != openai.FinishReasonStop {
        ai.log.Error("Error generating AI reply due to failure finish reason: %s", jsonUtil.AnyToJson(response.Choices[0]))
        return response.Choices[0].Message.Content, errors.New(response.Choices[0].Message.Content)
    }

    // response format: https://platform.openai.com/docs/guides/gpt/completions-response-format

    ai.log.Infof("AI reply used %d input tokens and %d output tokens", response.Usage.PromptTokens, response.Usage.CompletionTokens)

    return response.Choices[0].Message.Content, nil
}

func newGptClient() *openai.Client {
    secrets := secret.GetSecrets()
    return openai.NewClient(secrets.GptApiKey)
}
