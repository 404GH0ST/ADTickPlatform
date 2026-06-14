# Sistem Scoring — ADTickPlatform

> Dokumen untuk slide **Technical Meeting**. Menjelaskan cara kerja scoring
> ADTickPlatform: attack, defense, SLA, dan total. Dirancang untuk dibaca
> berurutan dari atas ke bawah; tiap section bisa dijadikan 1 slide.

---

## 1. Skema Besar

Skor tiap tim dihitung ulang setiap kali **Recompute Scoreboard** dipicu
(setiap submit flag valid atau tiap tick advance, melalui debounced
recompute). Terdiri dari **tiga komponen**:

| Komponen | Simbol | Sumber | Bisa negatif? |
|---|---|---|---|
| **Attack** | `A` | Submit flag musuh yang berhasil | Tidak (≥ 0) |
| **Defense** | `D` | Flag tim sendiri yang dicuri tim lain | **Ya** (≤ 0) |
| **SLA** | `S` | Hasil checker `put/get/check` per tick | Tidak (≥ 0) |
| **Total** | `T` | `T = A + D + S` | **Ya** |

**Rumus total:**

```text
T(team) = A(team) + D(team) + S(team)
```

Defense bisa membuat total negatif (tim yang bobrok). Tidak ada clamp ke 0.

---

## 2. Attack (A)

**Trigger:** peserta submit flag musuh via `POST /api/v2/submit`, dan
game-core memvalidasi flag itu (signature HMAC valid, tick belum expired,
bukan flag tim sendiri).

**Decay:** makin banyak tim yang menangkap **flag yang sama**, makin
kecil poin attack yang diberikan ke tiap penangkap. Flag yang hanya
bisa dicuri satu tim = paling berharga.

**Rumus (per flag, per submitter):**

```text
A(flag, submitter) = 1.0 + 1 / capture_count
```

di mana `capture_count` = jumlah total submitter unik untuk flag itu
(termasuk submitter saat ini). Flag yang sudah expired tidak dihitung.

**Tabel decay untuk satu flag:**

| captureCount | Poin attack per submitter | Total attack untuk flag itu |
|---:|---:|---:|
| 1 (eksklusif) | **2.000** | 2.000 |
| 2 | 1.500 | 3.000 |
| 3 | 1.333 | 4.000 |
| 4 | 1.250 | 5.000 |
| 8 | 1.125 | 9.000 |
| 16 | 1.0625 | 17.000 |

**Kode:** `faustAttackValue()` di `services/game-core/store.go`.

---

## 3. Defense (D)

**Trigger:** flag tim ini dicuri (submitted) oleh satu atau lebih tim
lawan. Penalty dikurangkan dari skor tim korban per flag yang
terkompromi.

**Rumus (per flag milik tim yang dicuri):**

```text
D(flag) = - capture_count ^ 0.75
```

di mana `capture_count` = jumlah submitter unik untuk flag itu. Penalty
naik **sub-linear** — capture ke-2 menambah penalty ~0.68, capture ke-3
~0.60, dst.

**Tabel penalty per flag:**

| captureCount | Penalty (dikurangkan dari D) |
|---:|---:|
| 1 | −1.000 |
| 2 | −1.682 |
| 3 | −2.280 |
| 4 | −2.828 |
| 8 | −4.757 |
| 16 | −8.000 |

**Implikasi:** tim yang punya 1 service dengan 8 attacker flag captures
kehilangan poin lebih besar dari 2 service dengan 4 captures masing-masing
(−4.757 vs −2×2.828 = −5.657). Spread across services membantu.

**Kode:** `faustDefensePenalty()` di `services/game-core/store.go`.

---

## 4. SLA (S)

**Trigger:** tiap tick, checker menjalankan fase `check` dengan input
normal ke tiap service instance (tiap tim × tiap challenge). Hasil
`check` diklasifikasikan jadi status:

| Service state | Nilai |
|---|---:|
| `ok` | 1.0 |
| `recovering` | 0.5 |
| lain (down, error, unknown) | 0.0 |

**Rumus:**

```text
S(team) = sum_over_services(value(state)) × √(team_count)
```

**Langkah 1 — sum per tim:**
Jumlahkan nilai state untuk semua service instance tim itu
(team × challenge pairs yang aktif).

**Langkah 2 — kalikan dengan team count factor:**
Kalikan total dengan `√(jumlah_tim)`. Rationale: makin banyak tim di
match, makin penting tiap service reliable untuk ekonomi match
keseluruhan.

**Contoh (4 tim, 3 challenges):**

| Tim | banking | chat | storage | Sum (sebelum factor) | SLA final |
|---|---|---|---|---:|---:|
| Alpha | 1.0 | 1.0 | 1.0 | 3.0 | 3.0 × √4 = **6.000** |
| Delta | 0.0 | 1.0 | 0.5 | 1.5 | 1.5 × √4 = **3.000** |
| Sigma | 1.0 | 0.0 | — | 1.0 | 1.0 × √4 = **2.000** |

**Kode:** `faustSLAValueForStatus()` + `faustSLAFactor()` di
`services/game-core/store.go`.

---

## 5. Worked Example — Satu Match

Setup: 4 tim, 3 challenges (banking, chat, storage), semua weight
sama (= 1, tidak ada multiplier), 2 tick berjalan.

### Tick 1

- Sigma men-submit 1 flag dari Delta (banking).
- Orchid men-submit 1 flag dari Delta (banking) — flag yang sama.
- Alpha & Delta semua service `ok`.

| Tim | Attack | Defense | SLA | Total |
|---|---:|---:|---:|---:|
| Alpha | 0.000 | 0.000 | 6.000 | **6.000** |
| Delta | 0.000 | −1.682 | 6.000 | **4.318** |
| Sigma | 1.500 | 0.000 | 6.000 | **7.500** |
| Orchid | 1.500 | 0.000 | 6.000 | **7.500** |

Catatan: flag yang sama dicuri 2 tim → `capture_count = 2`,
`A = 1 + 1/2 = 1.5` per submitter, dan Delta kehilangan `2^0.75 = 1.682`.

### Tick 2

- Sigma & Orchid men-submit flag dari Delta (chat) — flag yang sama.
- Alpha men-submit 1 flag dari Sigma (banking).
- Sigma & Delta service storage `down` (SLA turun).

| Tim | Attack | Defense | SLA | Total |
|---|---:|---:|---:|---:|
| Alpha | 2.000 + 0 = 2.000 | 0.000 | 6.000 | **8.000** |
| Delta | 0.000 | −1.682 + −1.682 = −3.364 | 3.000 | **−0.364** |
| Sigma | 1.500 + 2.000 = 3.500 | −1.000 | 4.000 | **6.500** |
| Orchid | 1.500 | 0.000 | 6.000 | **7.500** |

Perubahan tick 2 (delta dari tick 1):

- Alpha: +2.0 attack (menangkap flag Sigma), SLA tetap 6.0 → +2.0 total
- Delta: −1.682 defense (chat dicuri), SLA turun ke 3.0 (storage down) → −4.682
- Sigma: +1.5 attack (menangkap flag Delta chat), +2.0 attack (banking Alpha), −1.0 defense (banking dicuri Alpha), SLA turun ke 4.0 → −1.0
- Orchid: +1.5 attack (Delta chat) → +1.5

---

## 6. Why It Works — Design Rationale

**Decay attack (`1 + 1/capture`):** flag eksklusif bernilai 2× lipatnya
flag yang sudah diketahui 2 tim. Reward "skill + speed" — penangkap
pertama dapat paling banyak. Makin banyak tim tahu caranya, makin
murah nilainya.

**Sub-linear defense (`capture^0.75`):** satu capture menambah penalty
~0.75 (konstan). Tidak ada "cliff" di capture ke-N; progression
halus sehingga patching yang efektif (menurunkan capture count dari
8 ke 2) selalu worth it.

**SLA dengan `√(team_count)`:** match dengan 16 tim punya bobot SLA
4× lebih besar dari match 4 tim. Menyelaraskan reward dengan ukuran
ekosistem — service lebih penting saat lebih banyak yang bergantung
padanya.

**Decoupled attack/defense/SLA:** peserta bisa fokus ke salah satu
(sumbu) atau semuanya. Tim yang defensive (SLA tinggi, defense kecil)
bisa menang tanpa harus attack balik. Tim aggressive (attack tinggi)
bisa menang walau SLA anjlok. Spektrum strategi terbuka lebar.

---

## 7. Edge Cases & Clarifications

| Situasi | Yang terjadi |
|---|---|
| Submit flag milik sendiri | Ditolak, `status: invalid` (tidak masuk hitungan) |
| Submit flag duplikat | Ditolak, `status: duplicate` (tidak double-count) |
| Submit flag setelah expired | Ditolak, `status: invalid` (tidak dihitung) |
| Flag dicuri 0 tim | Defense = 0 untuk flag itu (bukan "no penalty") |
| Service `recovering` | Hitung setengah (0.5), bukan penuh (1.0) |
| Match paused / finished | Submit ditolak (match state check) |
| Warm-up tick pertama | SLA `ok` di tick pertama TIDAK dihitung (warm-up window) |

---

## 8. Recompute & Audit

**Recompute:** dipicu debounced setelah submit flag valid (debounce 1
detik, default). Bisa juga dipicu manual via
`POST /api/v2/admin/game/scoring/recompute`.

**Audit:** `GET /api/v2/admin/game/scoring/audit` menjalankan replay
dari nol dan membandingkan hasil dengan state tersimpan. Cocok untuk
verifikasi integritas skor saat dispute.

**Cache:** scoreboard terbaru di-serve dari memory store (singleton);
postgres store recompute on demand + cache last result.

---

## 9. Reference: Kode Sumber

Semua formula scoring berada di **`services/game-core/store.go`**
(bukan di api-gateway atau scoring-worker — scoring-worker hanya
proxy ke game-core untuk load distribution).

| Fungsi | Baris | Formula |
|---|---|---|
| `faustAttackValue` | 2251 | `1.0 + 1/capture_count` |
| `faustDefensePenalty` | 2258 | `capture_count^0.75` |
| `faustSLAValue` | 2265 | 1.0 / 0.5 / 0.0 by phase |
| `faustSLAValueForStatus` | 2276 | 1.0 / 0.5 / 0.0 by status |
| `faustSLAFactor` | 2287 | `√(team_count)` |

Postgres implementation menggunakan SQL yang equivalent (lihat
`buildScoreboard` di `services/game-core/store.go`).

---

*Versi: ADTickPlatform v1 — Technical Meeting briefing. Pertanyaan
atau dispute scoring: hubungi organizer via kanal resmi.*
