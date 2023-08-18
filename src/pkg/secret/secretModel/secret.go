package secretModel

type Secrets struct {
    SlackToken               string `json:"SlackToken"`
    NewUserSlackBotChannelId string `json:"NewUserSlackBotChannelId"`
    LineChannelSecret        string `json:"LineChannelSecret"`
    LineChannelAccessToken   string `json:"LineChannelAccessToken"`
    GptApiKey                string `json:"GptApiKey"`
    GoogleClientID           string `json:"GoogleClientID"`
    GoogleClientSecret       string `json:"GoogleClientSecret"`
}
