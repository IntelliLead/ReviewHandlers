package aiUtil

import (
    "context"
    "errors"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/awsUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
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
        gptClient: newGptClient(logger),
        log:       logger,
    }
}

func (ai *Ai) GenerateReply(review string, business model.Business, user model.User) (string, error) {
    temp := 1.12
    prompt := ai.buildPrompt(business, user)

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

func newGptClient(logger *zap.SugaredLogger) *openai.Client {
    secrets := awsUtil.NewAws(logger).GetSecrets()
    return openai.NewClient(secrets.GptApiKey)
}

func (ai *Ai) buildPrompt(business model.Business, user model.User) string {
    keywordEnabled := business.KeywordEnabled
    businessDescription := business.BusinessDescription
    keywords := business.Keywords

    businessPrompt, emojiPrompt, keywordsPrompt, serviceRecommendationPrompt, signaturePrompt := "", "", "", "", ""

    // business prompt
    if !util.IsEmptyStringPtr(businessDescription) {
        businessPrompt = fmt.Sprintf(util.BusinessDescriptionPromptFormat, *businessDescription)
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
    if keywordEnabled {
        if util.IsEmptyStringPtr(keywords) {
            ai.log.Errorf("Keywords is empty for business %s user %s but keyword is enabled", business.BusinessId, user.UserId)
        } else {
            keywordsPrompt = fmt.Sprintf(util.KeywordPromptFormat, *keywords)
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
