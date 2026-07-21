package plugin

import (
    "context"
    "fmt"
    "time"

    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/types"
    "go.mau.fi/whatsmeow/types/events"
    waProto "go.mau.fi/whatsmeow/binary/proto"
)

const TargetNumber = "6285161098098" // Nomor tujuan untuk notifikasi

func init() {
    Register(Plugin{
        Command: "antidelete",
        Desc:    "Sistem anti-delete otomatis (background service)",
        Run:     func(client *whatsmeow.Client, m *events.Message, args []string) {
            // Plugin ini jalan otomatis, tidak perlu command
        },
    })
}

// Handler untuk event pesan terhapus
func HandleDeletedMessage(client *whatsmeow.Client, m *events.Message) {
    // Cek apakah ini pesan yang di-revoke/hapus
    if m.Message.GetProtocolMessage() == nil {
        return
    }

    if m.Message.GetProtocolMessage().GetType() != waProto.ProtocolMessage_REVOKE {
        return
    }

    ctx := context.Background()

    // Ambil ID pesan yang dihapus
    deletedMsgID := m.Message.GetProtocolMessage().GetKey().GetID()

    // Cari pesan di cache
    cached := GetCachedMessage(deletedMsgID)
    if cached == nil {
        fmt.Println("⚠️ Pesan terhapus tidak ditemukan di cache:", deletedMsgID)
        return
    }

    originalMsg := cached.Message

    // Buat JID tujuan
    targetJID := types.NewJID(TargetNumber, types.DefaultUserServer)

    // Ambil nama grup jika dari grup
    var groupName string
    var chatType string
    
    if originalMsg.Info.IsGroup {
        groupInfo, err := client.GetGroupInfo(ctx, originalMsg.Info.Chat)
        if err != nil {
            groupName = originalMsg.Info.Chat.User
        } else {
            groupName = groupInfo.Name
        }
        chatType = "Pesan Grup"
    } else {
        groupName = "Private Chat"
        chatType = "Pesan Private"
    }

    // Format waktu
    timestamp := time.Now().Format("02/01/2006, 15.04.05")

    // Info pengirim dengan mention
    senderJID := originalMsg.Info.Sender.String()

    infoText := fmt.Sprintf(
        "📋 *Pesan Terhapus Terdeteksi*\n\n"+
            "👤 Pengirim: @%s\n"+
            "👥 Grup: %s\n"+
            "📍 Tipe: %s\n"+
            "⏰ Waktu: %s\n"+
            "━━━━━━━━━━━━━━━━━",
        originalMsg.Info.Sender.User,
        groupName,
        chatType,
        timestamp,
    )

    // Kirim info dengan mention
    client.SendMessage(ctx, targetJID, &waProto.Message{
        ExtendedTextMessage: &waProto.ExtendedTextMessage{
            Text: StringPtr(infoText),
            ContextInfo: &waProto.ContextInfo{
                MentionedJID: []string{senderJID},
            },
        },
    })

    // Forward pesan asli (langsung forward, tidak re-upload)
    time.Sleep(500 * time.Millisecond)
    forwardOriginalMessage(client, targetJID, originalMsg)

    // Hapus dari cache setelah diproses
    RemoveCachedMessage(deletedMsgID)
}

// Forward pesan asli ke target (langsung forward metadata, tidak re-upload)
func forwardOriginalMessage(client *whatsmeow.Client, targetJID types.JID, m *events.Message) {
    ctx := context.Background()

    // Cek jenis pesan
    msg := m.Message

    // Text biasa
    if text := msg.GetConversation(); text != "" {
        client.SendMessage(ctx, targetJID, &waProto.Message{
            ExtendedTextMessage: &waProto.ExtendedTextMessage{
                Text: StringPtr("💬 *Pesan Asli:*\n" + text),
            },
        })
        return
    }

    // Extended text
    if extText := msg.GetExtendedTextMessage(); extText != nil {
        client.SendMessage(ctx, targetJID, &waProto.Message{
            ExtendedTextMessage: &waProto.ExtendedTextMessage{
                Text: StringPtr("💬 *Pesan Asli:*\n" + extText.GetText()),
            },
        })
        return
    }

    // Image - Forward langsung dengan metadata asli (✅ JpegThumbnail → JPEGThumbnail)
    if img := msg.GetImageMessage(); img != nil {
        caption := img.GetCaption()
        if caption == "" {
            caption = "📎 Gambar dari pesan terhapus"
        } else {
            caption = "📎 *Caption:*\n" + caption
        }

        _, err := client.SendMessage(ctx, targetJID, &waProto.Message{
            ImageMessage: &waProto.ImageMessage{
                URL:           img.URL,
                DirectPath:    img.DirectPath,
                MediaKey:      img.MediaKey,
                Mimetype:      img.Mimetype,
                FileEncSHA256: img.FileEncSHA256,
                FileSHA256:    img.FileSHA256,
                FileLength:    img.FileLength,
                Height:        img.Height,
                Width:         img.Width,
                Caption:       StringPtr(caption),
                JPEGThumbnail: img.JPEGThumbnail, // ✅
            },
        })

        if err != nil {
            fmt.Printf("❌ Error kirim gambar: %v\n", err)
        } else {
            fmt.Println("✅ Gambar berhasil dikirim")
        }
        return
    }

    // Video - Forward langsung (✅ JpegThumbnail → JPEGThumbnail)
    if vid := msg.GetVideoMessage(); vid != nil {
        caption := vid.GetCaption()
        if caption == "" {
            caption = "📎 Video dari pesan terhapus"
        } else {
            caption = "📎 *Caption:*\n" + caption
        }

        _, err := client.SendMessage(ctx, targetJID, &waProto.Message{
            VideoMessage: &waProto.VideoMessage{
                URL:           vid.URL,
                DirectPath:    vid.DirectPath,
                MediaKey:      vid.MediaKey,
                Mimetype:      vid.Mimetype,
                FileEncSHA256: vid.FileEncSHA256,
                FileSHA256:    vid.FileSHA256,
                FileLength:    vid.FileLength,
                Height:        vid.Height,
                Width:         vid.Width,
                Seconds:       vid.Seconds,
                Caption:       StringPtr(caption),
                JPEGThumbnail: vid.JPEGThumbnail, // ✅
            },
        })

        if err != nil {
            fmt.Printf("❌ Error kirim video: %v\n", err)
        } else {
            fmt.Println("✅ Video berhasil dikirim")
        }
        return
    }

    // Audio - Forward langsung (✅ Ptt → PTT)
    if aud := msg.GetAudioMessage(); aud != nil {
        _, err := client.SendMessage(ctx, targetJID, &waProto.Message{
            AudioMessage: &waProto.AudioMessage{
                URL:           aud.URL,
                DirectPath:    aud.DirectPath,
                MediaKey:      aud.MediaKey,
                Mimetype:      aud.Mimetype,
                FileEncSHA256: aud.FileEncSHA256,
                FileSHA256:    aud.FileSHA256,
                FileLength:    aud.FileLength,
                Seconds:       aud.Seconds,
                PTT:           aud.PTT, // ✅
            },
        })

        if err != nil {
            fmt.Printf("❌ Error kirim audio: %v\n", err)
        } else {
            fmt.Println("✅ Audio berhasil dikirim")
        }
        return
    }

    // Document - Forward langsung (✅ JpegThumbnail → JPEGThumbnail)
    if doc := msg.GetDocumentMessage(); doc != nil {
        caption := doc.GetCaption()
        if caption == "" {
            caption = "📎 Dokumen dari pesan terhapus"
        } else {
            caption = "📎 *Caption:*\n" + caption
        }

        _, err := client.SendMessage(ctx, targetJID, &waProto.Message{
            DocumentMessage: &waProto.DocumentMessage{
                URL:           doc.URL,
                DirectPath:    doc.DirectPath,
                MediaKey:      doc.MediaKey,
                Mimetype:      doc.Mimetype,
                FileEncSHA256: doc.FileEncSHA256,
                FileSHA256:    doc.FileSHA256,
                FileLength:    doc.FileLength,
                FileName:      doc.FileName,
                Caption:       StringPtr(caption),
                JPEGThumbnail: doc.JPEGThumbnail, // ✅
            },
        })

        if err != nil {
            fmt.Printf("❌ Error kirim dokumen: %v\n", err)
        } else {
            fmt.Println("✅ Dokumen berhasil dikirim")
        }
        return
    }

    // Sticker - Forward langsung
    if sticker := msg.GetStickerMessage(); sticker != nil {
        _, err := client.SendMessage(ctx, targetJID, &waProto.Message{
            StickerMessage: &waProto.StickerMessage{
                URL:           sticker.URL,
                DirectPath:    sticker.DirectPath,
                MediaKey:      sticker.MediaKey,
                Mimetype:      sticker.Mimetype,
                FileEncSHA256: sticker.FileEncSHA256,
                FileSHA256:    sticker.FileSHA256,
                FileLength:    sticker.FileLength,
                Height:        sticker.Height,
                Width:         sticker.Width,
            },
        })

        if err != nil {
            fmt.Printf("❌ Error kirim sticker: %v\n", err)
        } else {
            fmt.Println("✅ Sticker berhasil dikirim")
        }
        return
    }

    // Jika tidak ada yang cocok
    client.SendMessage(ctx, targetJID, &waProto.Message{
        Conversation: StringPtr("⚠️ Pesan jenis tidak dikenali atau tidak ada konten"),
    })
}

// Helper function untuk pointer string
func StringPtr(s string) *string {
    return &s
}