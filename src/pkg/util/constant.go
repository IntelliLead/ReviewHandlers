package util

import (
    "fmt"
)

func HelpMessage() string {
    text := fmt.Sprint("本服務目前僅用於回覆Google Maps 評論。\n" +
        "回覆最新評論：使用評價訊息下方\"快速回覆\"按鈕即可編輯回覆內容。\n\n" +
        "若需回覆非最新評論：評論皆有編號，請在回覆時以 @編號 作為開頭。例如，如果評論編號為\"@8F\"，則回覆\"@8F 感謝您的認可！\"\n\n" +
        "若需更新回覆內容：以 @編號 作為開頭照常回覆即可。\n" +
        "新星評（無評論內容）也可以回覆。\n\n" +
        "新評論會在2分鐘內會推送到這裡。\n" +
        "評論者更新自己的已留評論不會被推送。\n\n" +
        "如需更多幫助，請聯係我們：")
    return text + "https://line.me/R/ti/p/%40006xnyvp"
}

func MoreMessage() string {
    url := "https://line.me/R/ti/p/%40006xnyvp"
    text := fmt.Sprint("更多功能開發中，敬請期待。\n\n" +
        "下一功能：Coming Soon...\n" +
        "智引力企劃的最新功能會在這邊不定期更新，歡迎查看！\n\n" +
        "Customer Obsession 是我們的 DNA。我們承諾持續聆聽用戶聲音，力圖為願意給我們機會的您提供最好的服務。\n\n" +
        "希望您不吝嗇提供寶貴建議，或若您有任何需要協助，都請隨時聯係我們：")
    return text + url + "\n我們將第一時間回覆。智引力感激您的支持！"
}

const DefaultUniqueId = "#"

const StageEnvKey = "STAGE"

const UpdateQuickReplyMessageCmd = "QuickReply"
const UpdateQuickReplyMessageCmdPrefix = "/" + UpdateQuickReplyMessageCmd + " "

const AiReplyPrompt = "You are a humble small business owner in Taiwan. You will be provided a customer review of your business. You will infer your exact business from the user's review, and reply in Taiwanese mandarin following best practices:\n- Be nice and don’t get personal. Keep your responses useful, readable, and courteous. - Keep it short and sweet under 200 characters. Don't need to begin by addressing the customer. Customers are looking for useful and genuine responses, but they can easily be overwhelmed by a long response.\n- Thank your reviewers\n- Be a friend, not a salesperson. Your reviewers are already customers, so there’s no need to offer incentives or advertisements. \n\nFor negative reviews:\n- suggest that they contact you personally by email or phone to resolve the issue. A positive post-review interaction and your reply shows prospective shoppers that you really care and often leads the customer to update their original review.\n- Be honest. Acknowledge mistakes that were made, but don’t take responsibility for things that are out of your control. Explain what you can and can't do in the situation. Show how you can make uncontrollable issues actionable. For example, bad weather caused you to cancel an event, but you monitor the weather and provide advance cancellation warnings when possible.\n- Apologize when appropriate. It’s best to say something that demonstrates compassion and empathy.\n- Show that you’re a real person by signing off with your name or initials. This helps you come across as more authentic."
