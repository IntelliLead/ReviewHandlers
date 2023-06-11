package util

import "fmt"

func HelpMessage() string {
    text := fmt.Sprint("本服務目前僅用於回復Google Maps 評價。\n" +
        "回復最新評價：使用評價訊息下方\"快速回復\"按鈕即可編輯回復內容。\n\n" +
        "若需回復非最新評價：評價皆有編號，請在回復時以 @編號 作為開頭。例如，如果評價編號為\"@8F\"，則回復\"@8F 感謝您的認可！\"\n\n" +
        "若需更新回復內容：以 @編號 作為開頭照常回復即可。\n" +
        "新星評（無評價內容）也可以回復。\n\n" +
        "新評價會在2分鐘內會推送到這裡。\n" +
        "評價者更新自己的已留評價不會被推送。\n\n" +
        "如需更多幫助，請聯係我們：")
    return text + "https://line.me/R/ti/p/%40006xnyvp"
}

const SlackToken = "xoxb-5347341178258-5407525762004-7sUyTmq2kNTrK3NkkDLPgGAS"
const NewUserBotChannelId = "C05BAB5MTL7"
