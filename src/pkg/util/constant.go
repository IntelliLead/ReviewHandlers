package util

import (
    "fmt"
)

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

// // prod
// const LineChannelSecret = "aa8c492c6295d7e3857fca4b41f49604"
// const LineChannelAccessToken = "AqTNC1x18DT0/e1rkVUEnigmwyyHj4cPa+TbX1ECE5NVfzeB7OPLUsQjRkXrbCzBp7etk9Skni4/8NZW9dBR6eDbeKTA+4CNFOtHEF5sHp+1nXDJ2dzQnuf/NV0vuqMju7iznWvpLaSGKbRonLs6FgdB04t89/1O/w1cDnyilFU="
//

// beta
const LineChannelSecret = "1866316d011430ce4c45a71fabb223fe"
const LineChannelAccessToken = "8pZz9bm5kl+WJca6GMLPili2GWwyruS7br3tfZV/0Srv9RdoH2Lzm5aCFwe4S90eYKDJCpusID7gkF2wYU9wvfnzMDvWAqTx2nV/LEaB8rPCTgznMWx/+J+cnMdjtug51WElkKunSNjvqHgKHtaGvAdB04t89/1O/w1cDnyilFU="

// gamma
// const LineChannelSecret = "23328da1156703fd124c5950fe7d8db4"
// const LineChannelAccessToken = "IJjTnHrFY7m4DrBW7DXSVLuZ3smB8SQlit3Zich/8gr4JfsI+VE3k+vcN4OjRySoS+fQb7FrdZUYH6VXo838LipHQstlofGrWRRUtgDhlRAi2c8UPxsQ5hkmkucWbkHd4yi3Mpc6I5q58QL5R/KrpgdB04t89/1O/w1cDnyilFU="
