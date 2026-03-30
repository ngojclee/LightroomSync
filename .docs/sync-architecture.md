# Lightroom Sync — Kiến trúc Đồng bộ

> **Version**: 2.0.1.0  
> **Last Updated**: 2026-03-30  
> **Status**: Đã thống nhất hướng giải quyết — sẵn sàng implement các cải tiến

---

## 1. Tổng quan

Hệ thống sử dụng **network share** (NAS/SMB) làm hub trung gian cho 2 luồng sync song song:

- **Catalog Sync**: Đồng bộ file `.lrcat` dạng zip backup — toàn bộ catalog  
- **Preset Sync**: Đồng bộ hai chiều từng file preset riêng lẻ

Không có cloud server. Mọi thứ qua file system.

```
┌──────────┐      \\NAS\Share\        ┌──────────┐
│  Máy A   │ ◄──── Catalog/ ────►    │  Máy B   │
│ (Studio) │ ◄──── Presets/ ────►    │ (Laptop) │
│          │       Presets/Logos/     │          │
└──────────┘                         └──────────┘
```

---

## 2. Catalog Sync

### 2.1 Manifest — File mốc (source of truth)

Khi một máy **backup xong** catalog (tạo zip), nó ghi `sync_manifest.json` trên network:

```json
{
  "machine": "DESKTOP-STUDIO",
  "timestamp": "2026-03-30T10:15:30.123456",
  "zip_file": "DESKTOP-STUDIO_20260330_101530.zip",
  "zip_size": 52428800
}
```

| Trường | Ý nghĩa |
|--------|---------|
| `machine` | Hostname máy đã tạo backup |
| `timestamp` | Thời điểm backup (ISO format, microsecond precision) |
| `zip_file` | Đường dẫn relative tới file zip |
| `zip_size` | Kích thước zip (byte) — để verify integrity |

### 2.2 Ba luật chống đồng bộ ngược (Anti-Self-Sync)

Hàm [`ShouldSyncFromNetwork()`](../internal/sync/manifest.go) kiểm tra 3 rules tuần tự:

| # | Rule | Điều kiện bỏ qua | Lý do |
|---|------|-------------------|-------|
| 1 | **Self-backup** | `manifest.machine == hostname máy hiện tại` | Không tự sync backup của chính mình |
| 2 | **Already synced** | `config.last_synced_timestamp >= manifest.timestamp` | Đã sync bản này rồi |
| 3 | **Zip integrity** | File zip không tồn tại hoặc size ≠ manifest | Backup chưa copy xong hoặc bị corrupt |

### 2.3 Khi nào chạy sync?

- **Startup**: Agent khởi động → check manifest ngay
- **Pending mode**: Nếu Lightroom đang mở → chờ Lightroom đóng mới sync
- **Watchdog**: Theo dõi liên tục file manifest thay đổi

---

## 3. Preset Sync

### 3.1 Cơ chế: So sánh mtime (two-way)

Không dùng manifest cho presets. So sánh trực tiếp **modification time** giữa local và network:

```
PULL: Nếu network.mtime > local.mtime + 2s → copy về local
PUSH: Nếu local.mtime > state.last_seen + 2s → push lên network
```

Tolerance 2 giây để tránh false conflict do filesystem timestamp resolution.

### 3.2 State file (`preset_state.json`)

Lưu tại `%APPDATA%\Adobe\Lightroom\.lightroom-sync\preset_state.json`.
Dùng để phát hiện file đã bị xóa (state có nhưng network/local không có).

---

## 4. Watermark Logo — Cơ chế (Ver 2.0)

**Mục tiêu**: Khi thêm preset watermark mới trên máy A, máy B tải về phải tự động nạp được logo, kể cả khi 2 máy có Username Windows khác nhau. Khi đổi logo file trên mạng, các máy khác cũng tự cập nhật.

### 4.1 Vị trí lưu trữ (Storage Paths) - [Phương án A]
- **Trên máy tính (Local)**: `%APPDATA%\Adobe\Lightroom\Watermarks\Logos\`
- **Trên Server (Network)**: `\\NAS\Share\Presets\Watermarks\Logos\`
*(Cố ý đặt bên trong thư mục `Watermarks` ở cả 2 bên để giữ nguyên cấu trúc phân cấp thẩm mỹ của Lightroom)*

### 4.2 Giải pháp thống nhất
1. **PULL & REPLACE GẦN ĐÚNG**: Tải file logo từ network (`Presets\Watermarks\Logos\`) về ổ cứng nội bộ của máy trạm (lưu trong `Watermarks/Logos/`). Sau đó, tự động đổi đường dẫn (replace path) trong file preset `.lrtemplate` trỏ trực tiếp vào thư mục của máy trạm đó. *Lý do: Đảm bảo khi mất kết nối mạng NAS, người dùng vẫn có thể xuất ảnh đóng logo bình thường.*
2. **THEO DÕI THAY ĐỔI**: So sánh bản logo trên network và trên local thông qua **File Size** TRỘN VỚI **Modification Time (thời gian sửa file gốc)**. Nếu file logo trên NAS có thời gian sửa (modified time) hoặc kích thước mới hơn so với file local, máy tự động tải đè logo đó xuống.
3. **MƯỢT MÀ**: Path đã replace 1 lần rồi thì giữ nguyên ko cần replace lại, chỉ cần đè tệp ảnh .png.

### 4.3 Tránh xung đột kép (Conflict Avoidance)
**Vấn đề**: Vì thư mục `Watermarks` là một Category Preset mặc định tự đồng bộ, nếu ta lưu File ảnh Logo vào bên trong `Watermarks/Logos/` (ở cả Local và Network), App sẽ hiểu lầm tấm ảnh đó là một Preset và sẽ Cố Đồng Bộ Tấm Ảnh.
**Hành động**: Trong đoạn Code quét Preset Local (`scanPresetFiles`), bổ sung **Quy tắc Ngoại lệ**: Thấy thư mục con tên là **`Logos`** thì **SkipDir** không quét. Nhờ vậy, thư mục Logo do một mình Engine Logo độc quyền quản lý.

---

## 5. Phân tích lỗ hổng & Hướng giải quyết (Multi-machine scenarios)

### 5.1 ⚠️ Máy mới gia nhập (New Machine Onboarding) & Mất kết nối lâu ngày
**Vấn đề**: Máy C là máy tính mới cài đặt Lightroom Sync, hoặc một máy rất lâu không sử dụng có những chỉnh sửa ảnh riêng lẻ. Nếu bật tự động Sync, nó bị đè mất Catalog cũ hoặc đẩy Catalog lệch chuẩn lên chia cho các máy khác.
**Giải pháp**:
- **Trạng thái mặc định là "PAUSE"**: Cài đặt lần đầu, auto-sync Catalog mặc định sẽ DỪNG LẠI. 
- **Prompt Chọn Quyền (Master/Slave)**: Hiển thị ngay mục thông báo hướng dẫn để yêu cầu chọn:
  - *"Tải bản Catalog từ Network về đè máy này" (Máy mới trắng)*
  - *"Up bản Catalog máy này lên làm chuẩn cho toàn Network"*
- **Tab Hướng Dẫn Trực Quan (Onboarding Guide)**: Viết thêm 1 tab riêng hoàn toàn bằng Tiếng Anh (hoặc song ngữ), minh họa bằng hình và giải thích các bước: *"Nếu bạn sử dụng phần mềm lần đầu, nhưng máy đang có catalog riêng: Đầu tiên bạn mở Lightroom, chọn Export As Catalog cho những ảnh mới. Sau đó mới dùng app tải Catalog Gốc trên Server về, Mở lên, và Import from another Catalog - chọn cái catalog bạn vừa nháp để Nối (Merge) vào nhau, sau đó Catalog tự up lên lại mạng"* → Như vậy user 100% ko mất ảnh. 

### 5.2 ⚠️ Race condition (2 máy lưu cùng lúc) & Corruption
**Vấn đề**: Hai máy cùng ghi manifest, gây lỗi cấu trúc file.
**Giải pháp**:
- **Dùng File Lock Cơ Bản**: Sinh cấu trúc `manifest_lock` chặn máy khác thao tác khi đang copy file backup lên.
- **Phát hiện hỏng file (Corruption)**: Hiện đã dùng kĩ thuật so kích thước `zip_size` ở code Go và kiểm tra Headers. Tuy nhiên, nếu bị Corrupt, cần bổ sung **Bắn thông báo (Notification Tray)** cho User biết để họ chủ động tải lại (hoặc yêu cầu tạo lại backup từ máy khác) chứ không để phần mềm im im bỏ chọn.

### 5.3 ⚠️ Preset: Delete-then-push-back loop (Máy Offline)
**Vấn đề**: Preset bị xóa và Push thành công lên server. Tuy nhiên, Máy C đang mất mạng chưa kịp bắt nhịp. Lúc Máy C có mạng thì App tưởng nhầm Máy C "có Preset mới tinh", thành ra Máy C đẩy con Zombie sống lại cho toàn mạng.
**Giải pháp**:
- **"Tombstone" Marker (Bia Mộ)**: Quản lý File `sync_deleted.json` trên Network: Lưu danh sách tên file kèm thời gian bị chém. Bất cứ máy nào Sync, ngó qua bia mộ trước, thấy Timestamp của file mình Đang Cầm trên tay mà cũ hơn cả thời gian trên bia mộ thì máy nó tự giác thủ tiêu file thay vì UP load láo lên.

### 5.4 ⚠️ Preset: Simultaneous Edit Conflict
**Vấn đề**: Hai người sửa cùng 1 cái preset cùng thời điểm thì lấy kết quả nào.
**Giải pháp (Lấy Bản Mới Nhất Làm Chuẩn)**:
- Cơ chế của phần mềm sẽ tin vào người cuối cùng, khi đẩy preset, **chỉ cần Modification Time (Thời gian Sửa đổi Preset) của bản nào cao hơn (Mới hơn) thì lấy**. Đây là Last-writer-wins, hoàn toàn đúng logic không cần thông báo thừa khiến rắc rối. 
