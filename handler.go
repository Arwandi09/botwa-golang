package main

import (
    "strings"

    "botwa/log"
    "botwa/plugin"

    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/types/events"
)

func Handler(client *whatsmeow.Client) func(interface{}) {
    return func(evt interface{}) {
        switch m := evt.(type) {
        case *events.Message:
            // Log pesan
            log.Raw(m, "!")

            // Cek apakah ini pesan revoke/delete
            if m.Message.GetProtocolMessage() != nil {
                plugin.HandleDeletedMessage(client, m)
                return
            }

            // Download dan cache media (jika ada)
            mediaPath := plugin.DownloadAndCacheMedia(client, m)

            // Simpan pesan ke cache
            plugin.CacheMessage(m, mediaPath)

            // Proses command
            text := m.Message.GetConversation()
            if text == "" && m.Message.GetExtendedTextMessage() != nil {
                text = m.Message.GetExtendedTextMessage().GetText()
            }

            if strings.HasPrefix(text, Prefix) {
                plugin.Execute(client, m, text)
            }
        }
    }
}