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
    // var temp float32
    // var prompt string
    // if userId == util.NailSalonUserId || userId == util.AlphaUserId {
    //     temp = 1.12
    //     prompt = util.AiReplyPromptNailSalon
    // } else {
    //     temp = 1.0
    //     prompt = util.AiReplyPrompt
    // }

    temp := 1.12
    businessPrompt, keywordsPrompt := "", ""
    if !util.IsEmptyStringPtr(user.BusinessDescription) {
        businessPrompt = "Your business is" + *user.BusinessDescription + "."
    }
    if user.KeywordEnabled {
        if util.IsEmptyStringPtr(user.Keywords) {
            ai.log.Errorf("Keywords is empty for user %s but SEO is enabled", user.UserId)
        } else {
            keywordsPrompt = "- Try to mention all or parts of the following in a natural way: " + *user.Keywords + ".\n"
        }
    }

    prompt := fmt.Sprintf(util.AiReplyPromptFormat, businessPrompt, keywordsPrompt)

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
