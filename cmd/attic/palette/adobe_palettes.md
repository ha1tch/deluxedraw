## Introduction – Why bother with binary palette files?

At first glance, bundling **binary** palette formats (ACO/ASE) into a modern, text‑centric toolchain feels counter‑intuitive. We chose to support them anyway for three big reasons:

1. **Seamless Adobe interoperability** – Photoshop, Illustrator, InDesign, After Effects, and countless plug‑ins still treat ACO/ASE as the native palette currency. Shipping those files means zero extra clicks for the design team.
2. **Ubiquitous third‑party support** – Free editors like Photopea, Krita, Inkscape, Affinity, and even browser colour‑picker extensions can import them directly, so a single export covers the entire ecosystem.
3. **Deterministic, spec‑simple binaries** – Both formats are tiny, stable, and easy to generate reproducibly from our master JSON/Go palette. We keep the human‑readable colours in Git, auto‑build the binaries on release, and never risk divergence.

Bottom line: we stay **text‑first** in source control but deliver the exact artefacts creative suites expect—no manual conversions, no mismatched hues.

---

# Adobe Palette File Formats – Deep‑Dive

A practical, implementation‑oriented description of the two binary palette formats most widely accepted by Adobe apps and many third‑party tools.

---

## 1 Why two formats?

| Format                                  | First app             | Typical usage                                                                   | Max colours       | Supports names | Supports groups |
| --------------------------------------- | --------------------- | ------------------------------------------------------------------------------- | ----------------- | -------------- | --------------- |
| **ACO** (Adobe **Co**lor Swatch)        | Photoshop 5 (1998)    | Point‑sampled **colour swatches**, mainly RGB/CMYK                              | 4 096 per version | ✔ (v2 only)    | ✖               |
| **ASE** (Adobe **S**watch **E**xchange) | Illustrator CS (2003) | Portable **library palettes** across Illustrator, InDesign, After Effects, etc. | 16 777 215 blocks | ✔              | ✔ nested        |

*ACO* is simpler but older; *ASE* is newer, extensible, and officially preferred for cross‑app workflows.

---

## 2 Adobe Color Swatch **(ACO)**

### 2.1 High‑level layout

```
┌────────────┐
│  Version 1 │  – mandatory, without colour names
├────────────┤
│  Version 2 │  – optional; same colours again + UTF‑16BE names
└────────────┘
```

A file may contain **V1 only** or **V1 followed by V2**. Modern Photoshop ignores V2 if V1 is absent.

### 2.2 Binary grammar

| Offset | Type              | Description                              |
| ------ | ----------------- | ---------------------------------------- |
| 0      | uint16            | **Version** (1 or 2)                     |
| 2      | uint16            | **Count** *n* (number of colour records) |
| 4      | *repeat n* × 10 B | **Colour Structure** (see below)         |
| …      | V2 only           | Unicode names block per colour           |

**Colour Structure** – 10 bytes

| Field      | Type     | Meaning                                   |
| ---------- | -------- | ----------------------------------------- |
| colorSpace | uint16   | 0 RGB, 1 HSB, 2 CMYK, 7 Lab, 8 Gray,…     |
| v1…v4      | 4×uint16 | 16‑bit channel data (unused channels = 0) |

> **Scaling:** Adobe stores channels as integers in `0…65535`. 8‑bit RGB converts by `v16 = v8 × 257`.

**Names (Version 2)**

```
uint32 nameLen      // count of UTF‑16 code units *including* null
uint16[nameLen]     // UTF‑16BE text + 0x0000 terminator
```

### 2.3 Tips & pitfalls

* Always write **both** versions for maximum compatibility.
* `colorSpace==2` (CMYK) expects *un‑premultiplied* 16‑bit values.
* Name length is **code‑units**, not bytes.

---

## 3 Adobe Swatch Exchange **(ASE)**

### 3.1 Container model

ASE is a tiny TLV (Type‑Length‑Value) container. Everything is a **block**.

```
File Header
┌─────────────────────────────┐
│ Block 0 – maybe GroupStart │
├─────────────────────────────┤
│ Block 1 – Colour Entry      │
├─────────────────────────────┤
│ …                           │
└─────────────────────────────┘
```

### 3.2 File header – 12 bytes

| Offset | Size | Value                    |
| ------ | ---- | ------------------------ |
| 0      | 4 B  | ASCII **"ASEF"**         |
| 4      | 2 B  | Major ver. (0x0001)      |
| 6      | 2 B  | Minor ver. (0x0000)      |
| 8      | 4 B  | **Block count** (uint32) |

### 3.3 Block envelope

| Field     | Type   | Note                                                       |
| --------- | ------ | ---------------------------------------------------------- |
| blockType | uint16 | 0xC001 GroupStart<br>0xC002 GroupEnd<br>0x0001 ColourEntry |
| blockLen  | uint32 | Payload **length in bytes** (big‑endian)                   |
| payload   | bytes  | Format depends on type                                     |

### 3.4 Payloads

#### (a) Group Start / End

| Block     | Body                                      |
| --------- | ----------------------------------------- |
| **Start** | `uint16 nameLen` + UTF‑16BE name + 0x0000 |
| **End**   | *empty* (blockLen = 0)                    |

#### (b) Colour Entry

```
uint16 nameLen          // UTF‑16BE units incl. null
uint16[nameLen]
char[4] model           // "RGB ", "CMYK", "LAB ", "Gray"
float32[n] channels     // n=1,3,4 depending on model
uint16 colorType        // 0 Global, 1 Spot, 2 Normal
```

All floats and integers are **big‑endian IEEE‑754**.

### 3.5 Common mistakes

* `blockLen` **excludes** the 6‑byte envelope.
* Model tag is *padded with spaces* to exactly 4 bytes.
* Names **must** be null‑terminated UTF‑16BE.

---

## 4 Implementing writers in Go

1. **Endianness** – use `binary.BigEndian` for every integer/float.
2. **UTF‑16** – convert with `utf16.Encode([]rune(name))`, then write each `uint16`.
3. **Scaling**

   * ACO:  `uint16(v8) * 257` (equal‑spread)
   * ASE:  `float32(v8) / 255`  (0–1)
4. **Block book‑keeping** – build payloads in a `bytes.Buffer`, then prefix with size.
5. **Testing** – Photopea.com and Inkscape can open both formats without Adobe software.

---

## 5 Further reading

* *Adobe Photoshop File Formats Specification* (v6) – ACO details
* *Illustrator SDK – Swatch Exchange* appendix – early ASE note
* Reverse‑engineered spec by *Evan Wallace* and *Mitchell van Zuylen*

---

*Last updated: 13 July 2025*
