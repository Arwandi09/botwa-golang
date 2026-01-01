package log

import (
    "fmt"
    "strings"
    "time"

    "go.mau.fi/whatsmeow/types/events"
)

// Raw menerima prefix sebagai parameter
func Raw(m *events.Message, prefix string) {
    t := time.Now().Format("02/01/06 15:04:05")
    from := m.Info.Sender.String()
    name := m.Info.PushName
    chat := m.Info.Chat.String()

    text := ""

    // ambil isi pesan
    if m.Message.GetConversation() != "" {
        text = m.Message.GetConversation()
    } else if m.Message.GetExtendedTextMessage() != nil {
        text = m.Message.GetExtendedTextMessage().GetText()
    }

    // cek prefix command
    logType := "MSG"
    if prefix != "" && strings.HasPrefix(text, prefix) {
        logType = "CMD"
    }

    fmt.Printf(
        "[ %s ] %s  from [%s]  %s  in [%s]  %s\n",
        logType, t, from, name, chat, text,
    )
}
