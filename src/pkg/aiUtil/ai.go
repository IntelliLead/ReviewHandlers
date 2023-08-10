package aiUtil

import (
    "context"
    "errors"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
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

func (ai *Ai) GenerateReply(review string, user model.User) (string, error) {
    temp := 1.12

    prompt := ai.buildPrompt(user)
    // DEBUG
    ai.log.Debug("AI Reply Prompt: ", prompt)

    response, err := ai.gptClient.CreateChatCompletion(
        context.Background(),
        openai.ChatCompletionRequest{
            Temperature: float32(temp),
            MaxTokens:   256,
            Model:       openai.GPT4,
            Messages: []openai.ChatCompletionMessage{
                {
                    Role:    openai.ChatMessageRoleSystem,
                    Content: prompt,
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
                // TODO: [INT-62] add retry
            case 500:
                ai.log.Error("Error generating AI reply due to OpenAI internal server error: ", err)
                // TODO: [INT-62] add retry
            default:
                ai.log.Error("Error generating AI reply due to unknown error: ", err)
            }
        }

        return "", err
    }

    // response format: https://platform.openai.com/docs/guides/gpt/completions-response-format
    if response.Choices[0].FinishReason != openai.FinishReasonStop {
        ai.log.Error("Error generating AI reply due to failure finish reason: %s", jsonUtil.AnyToJson(response.Choices[0]))
        return response.Choices[0].Message.Content, errors.New(response.Choices[0].Message.Content)
    }

    ai.log.Infof("AI reply used %d input tokens and %d output tokens", response.Usage.PromptTokens, response.Usage.CompletionTokens)

    return response.Choices[0].Message.Content, nil
}

func newGptClient() *openai.Client {
    secrets := secret.GetSecrets()
    return openai.NewClient(secrets.GptApiKey)
}

func (ai *Ai) buildPrompt(user model.User) string {
    businessPrompt, emojiPrompt, keywordsPrompt, serviceRecommendationPrompt, signaturePrompt := "", "", "", "", ""

    // business prompt
    if !util.IsEmptyStringPtr(user.BusinessDescription) {
        businessPrompt = fmt.Sprintf(util.BusinessDescriptionPromptFormat, *user.BusinessDescription)
    }

    // emoji prompt
    if user.EmojiEnabled {
        emojiPrompt = util.EmojiPrompt
    }

    // service recommendation prompt
    if user.ServiceRecommendationEnabled {
        if util.IsEmptyStringPtr(user.ServiceRecommendation) {
            serviceRecommendationPrompt = fmt.Sprintf(util.ServiceRecommendationPromptFormat, "")
        } else {
            serviceRecommendationPrompt = fmt.Sprintf(
                util.ServiceRecommendationPromptFormat, fmt.Sprintf(util.ServiceToRecommendPromptFormat, *user.ServiceRecommendation))
        }
    }

    // keyword prompt
    if user.KeywordEnabled {
        if util.IsEmptyStringPtr(user.Keywords) {
            ai.log.Errorf("Keywords is empty for user %s but keyword is enabled", user.UserId)
        } else {
            keywordsPrompt = fmt.Sprintf(util.KeywordPromptFormat, *user.Keywords)
        }
    }

    // signature prompt
    if user.SignatureEnabled {
        if util.IsEmptyStringPtr(user.Signature) {
            ai.log.Errorf("Signature is empty for user %s but signature is enabled", user.UserId)
        } else {
            signaturePrompt = fmt.Sprintf(util.SignaturePrompt, *user.Signature)
        }
    }

    return fmt.Sprintf(util.AiReplyPromptFormat, businessPrompt, emojiPrompt, serviceRecommendationPrompt, keywordsPrompt, signaturePrompt)
}
