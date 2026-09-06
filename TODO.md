# Roadmap & TODO: SDK Messaging Kafka Enhancement

Dokumen ini merangkum rencana peningkatan (*enhancement*) pada modul `SDK/messaging/kafka` berdasarkan hasil komparasi dengan `Pegadaian/sdk/ist-oneframework-sdk/message_stream` dan kebutuhan operasional di lingkungan microservices Pegadaian.

---

## Ringkasan Prioritas

- [x] **Phase 1: Worker Resilience & Panic Safety** (Priority: **P0 - Critical**)
- [ ] **Phase 2: Pegadaian Envelope Interoperability & Codec** (Priority: **P1 - High**)
- [ ] **Phase 3: SASL Authentication & Config Ergonomics** (Priority: **P1 - High**)
- [ ] **Phase 4: Multi-Cluster Migration Support (Dual Consumer)** (Priority: **P2 - Medium**)
- [ ] **Phase 5: OpenTelemetry Semantic Spans & Payload Masking** (Priority: **P2 - Medium**)

---

## Detail Rencana Kerja

### Phase 1: Worker Resilience & Panic Safety (Priority: P0)
> **Tujuan**: Mencegah pod Kubernetes crash fatal saat user handler mengalami panic, serta memastikan consumer tidak mati mendadak saat terjadi gangguan jaringan sekejap (*transient network hiccup* atau *broker rebalance*).

- [x] **1.1. Panic Recovery pada Consumer Worker Goroutine**
  - Pasang `defer func() { if r := recover(); r != nil { ... } }()` di dalam worker goroutine `c.process(...)`.
  - Format stack trace panic dan log via `c.logger.Error`.
  - Jika `WithDeadLetterTopic` diaktifkan, teruskan pesan yang menyebabkan panic ke DLQ dengan header `x-error: panic: ...`.
  - Commit offset setelah masuk ke DLQ agar partisi tidak mengalami poison pill loop selamanya.

- [x] **1.2. Transient Fetch Error Resilience & Backoff**
  - Saat ini di `consumer.go:90`, jika `c.reader.FetchMessage(ctx)` gagal, consumer langsung exit.
  - Tambahkan tracking consecutive errors dan backoff delay (misal: sleep 500ms - 2s) untuk transient network error.
  - Berikan batas toleransi error berturut-turut (misal `maxConsecutiveErrors: 10`, dapat dikonfigurasi via `WithMaxConsecutiveErrors(n)`).
  - Abaikan timeout berkala (non-fatal) agar worker tidak berhenti saat traffic sepi.

- [x] **1.3. Unit Testing & Verifikasi Resilience**
  - Test handler yang memicu panic dan pastikan tidak men-crash consumer runner serta terkirim ke mock DLQ.
  - Test skenario transient fetch failure dan pemulihannya.

---

### Phase 2: Pegadaian Envelope Interoperability & Codec (Priority: P1)
> **Tujuan**: Memungkinkan service baru yang menggunakan `SDK/messaging/kafka` dapat saling berkirim dan menerima pesan dengan service eksisting Pegadaian yang menggunakan standar wrapper `{"value": ..., "context": {"metadata": {...}}}`.

- [ ] **2.1. Abstraksi Envelope / Codec**
  - Buat interface atau mekanisme codec untuk membedakan antara **Raw Payload** (standar Kafka murni) dan **Pegadaian Envelope**.
  - Struktur Pegadaian Envelope:
    ```json
    {
      "value": { ... },
      "context": {
        "metadata": {
          "trace_id": "...",
          "client_ip": "...",
          "session_id": "...",
          "header_list": { ... }
        }
      }
    }
    ```

- [ ] **2.2. Producer Pegadaian Envelope Support**
  - Tambahkan opsi `WithPegadaianEnvelope()` atau codec pada `Producer`.
  - Saat `Publish`, otomatis bungkus `msg.Payload` dan metadata context (`trace_id`, `client_ip`, dll.) ke dalam JSON envelope Pegadaian agar dapat diproses oleh consumer OneFramework lama.

- [ ] **2.3. Consumer Auto-Detection / Unwrapping**
  - Di sisi consumer, buat deteksi pintar (atau via opsi `WithEnvelopeCodec`): jika payload berformat envelope Pegadaian, ekstrak metadata ke `api_context` / tracing context dan kupas isi `value` ke `msg.Payload` murni sebelum handler dieksekusi.

- [ ] **2.4. Unit Test Interoperabilitas**
  - Test produce & consume bolak-balik antara format raw bytes dan format envelope Pegadaian.

---

### Phase 3: SASL Authentication & Config Ergonomics (Priority: P1)
> **Tujuan**: Mempermudah konfigurasi otentikasi Kafka (SCRAM / PLAIN) dari file environment/YAML tanpa mengharuskan developer mengimpor package low-level `segmentio/kafka-go/sasl`.

- [ ] **3.1. Shortcut Functional Options untuk SASL**
  - Tambahkan helper options di `options.go`:
    - `WithSASLPlain(username, password string) Option`
    - `WithSASLScramSHA256(username, password string) Option`
    - `WithSASLScramSHA512(username, password string) Option`
  - Validasi username dan password tidak boleh kosong jika opsi dipilih.

- [ ] **3.2. Unit Test SASL Options**
  - Verifikasi bahwa options menghasilkan mekanisme SASL yang tepat pada client dialer & transport.

---

### Phase 4: Multi-Cluster Migration Support (Priority: P2)
> **Tujuan**: Mendukung proses migrasi kluster Kafka di Pegadaian (Primary vs Secondary / Old vs New) tanpa downtime (*zero-downtime cutover*).

- [ ] **4.1. DualConsumer (Simultaneous Consumption)**
  - Sediakan abstraksi `DualConsumer` yang menerima dua instance `kafka.Consumer` (Old Cluster & New Cluster) dan mengeksekusi handler yang sama.
  - Memberikan sinyal shutdown serentak saat context dibatalkan.
  - Mirip fitur `ENABLE_KAFKA_DUAL_READ` di OneFramework namun dengan kontrol lifecycle yang bersih.

- [ ] **4.2. Switchable / Migration Producer**
  - Helper producer yang dapat dialihkan target clusternya secara dinamis atau berdasarkan konfigurasi flag cutover.

---

### Phase 5: OpenTelemetry Semantic Spans & Payload Masking (Priority: P2)
> **Tujuan**: Meningkatkan visibilitas APM/tracing dan keamanan log.

- [ ] **5.1. OpenTelemetry Semantic Messaging Spans**
  - Buat span OTel eksplisit pada saat `Publish` (`trace.SpanKindProducer`) dan `Consume` (`trace.SpanKindConsumer`).
  - Set atribut sesuai standar OpenTelemetry Messaging Conventions:
    - `messaging.system`: "kafka"
    - `messaging.destination.name`: topic
    - `messaging.kafka.message.offset`: offset
    - `messaging.kafka.message.key`: key

- [ ] **5.2. Debug Payload Logging with Data Masking**
  - Opsional logging payload body pada level `DEBUG` dengan masking field sensitif (password, PIN, token, NIK) menggunakan aturan dari `core/observability/logger`.
