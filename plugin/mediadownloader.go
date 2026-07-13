package plugin

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "time"

    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/types/events"
)

const MediaCacheDir = "media_cache"

func init() {
    // Buat folder cache jika belum ada
    os.MkdirAll(MediaCacheDir, 0755)
}

// Download dan cache media dari pesan
func DownloadAndCacheMedia(client *whatsmeow.Client, m *events.Message) string {
    ctx := context.Background()
    msg := m.Message

    var data []byte
    var err error
    var ext string
    var mediaType string

    // Image (✅ tambah ctx)
    if img := msg.GetImageMessage(); img != nil {
        data, err = client.Download(ctx, img)
        ext = ".jpg"
        mediaType = "image"
    } else if vid := msg.GetVideoMessage(); vid != nil {
        // Video
        data, err = client.Download(ctx, vid)
        ext = ".mp4"
        mediaType = "video"
    } else if aud := msg.GetAudioMessage(); aud != nil {
        // Audio
        data, err = client.Download(ctx, aud)
        ext = ".ogg"
        mediaType = "audio"
    } else if doc := msg.GetDocumentMessage(); doc != nil {
        // Document
        data, err = client.Download(ctx, doc)
        ext = filepath.Ext(doc.GetFileName())
        if ext == "" {
            ext = ".bin"
        }
        mediaType = "document"
    } else if sticker := msg.GetStickerMessage(); sticker != nil {
        // Sticker
        data, err = client.Download(ctx, sticker)
        ext = ".webp"
        mediaType = "sticker"
    } else {
        // Tidak ada media
        return ""
    }

    if err != nil {
        fmt.Println("❌ Error download media:", err)
        return ""
    }

    // Simpan file
    filename := fmt.Sprintf("%s_%s%s", m.Info.ID, mediaType, ext)
    filePath := filepath.Join(MediaCacheDir, filename)

    err = os.WriteFile(filePath, data, 0644)
    if err != nil {
        fmt.Println("❌ Error menyimpan file:", err)
        return ""
    }

    fmt.Printf("✅ Media disimpan: %s\n", filePath)
    return filePath
}

// Cleanup media cache lama (opsional)
func CleanupMediaCache(maxAgeDays int) {
    files, err := os.ReadDir(MediaCacheDir)
    if err != nil {
        return
    }

    now := time.Now()
    for _, file := range files {
        info, err := file.Info()
        if err != nil {
            continue
        }

        // Hapus file lebih dari X hari
        age := now.Sub(info.ModTime())
        if age > time.Duration(maxAgeDays)*24*time.Hour {
            filePath := filepath.Join(MediaCacheDir, file.Name())
            os.Remove(filePath)
            fmt.Printf("🗑️ Hapus cache lama: %s\n", file.Name())
        }
    }
}