# File Processing Flow - Pre-LLM Pipeline

## Overview
This document explains the complete data processing flow from file upload to LLM input generation. This is a critical pipeline that transforms raw business data (Mexican CFDI line items) into clean, deduplicated JSON ready for LLM classification.

---

## 🎯 High-Level Flow

```
1. File Upload → 2. Schema Inspection → 3. User Selection → 4. Refinery Cleaning → 5. Deduplication → 6. JSON Generation → [7. LLM Processing]
```

---

## 📤 Stage 1: File Upload & Storage

### Endpoint
`POST /api/pipeline/upload`

### Input Formats Supported
- **Excel**: `.xlsx` (parsed with `pd.read_excel()`)
- **CSV**: `.csv` (parsed with `pd.read_csv()`)
- **JSON**: `.json` (parsed with `pd.read_json()`)
- **JSON Lines**: `.jsonl`, `.jsonnl`, `.ndjson` (parsed line-by-line)

### What Happens

```python
# 1. Receive file from user
file = await request.form()["file"]

# 2. Read file into pandas DataFrame
if filename.endswith('.xlsx'):
    df = pd.read_excel(file.file)
elif filename.endswith('.csv'):
    df = pd.read_csv(file.file)
elif filename.endswith('.json'):
    df = pd.read_json(file.file)
elif filename.endswith(('.jsonl', '.jsonnl', '.ndjson')):
    df = pd.read_json(file.file, lines=True)

# 3. Generate unique upload_id
upload_id = str(uuid.uuid4())

# 4. Save DataFrame as pickle for fast subsequent access
file_path = f"/tmp/upload_{upload_id}.pkl"
df.to_pickle(file_path)
```

### Output
```json
{
  "upload_id": "abc-123-def-456",
  "columns": [
    {
      "name": "LineDescription",
      "dtype": "object",
      "null_count": 12,
      "sample_values": ["PROMO TV 2024", "MATERIAL POP", "EVENTO CDMX"]
    },
    {
      "name": "Amount",
      "dtype": "float64",
      "null_count": 0,
      "sample_values": [1500.50, 2300.00, 890.75]
    }
  ]
}
```

### Key Points
- **Format Agnostic**: All formats are converted to pandas DataFrame
- **Temporary Storage**: Files stored in `/tmp/upload_{upload_id}.pkl`
- **Pickle Format**: Binary format for fast read/write operations
- **No Data Loss**: Original data completely preserved at this stage
- **The user can pick multiple columns to clean in the next stage**

---

## 🔍 Stage 2: Schema Inspection & Column Analysis

### What Gets Analyzed

The system analyzes each column to provide metadata for user decision-making:

```python
# For each column in the DataFrame
for col in df.columns:
    column_info = {
        "name": col,                          # Column name
        "dtype": str(df[col].dtype),          # Data type (object, int64, float64, etc.)
        "null_count": int(df[col].isna().sum()),  # Number of null values
        "non_null_count": int(df[col].notna().sum()),
        "unique_values": int(df[col].nunique()),
        "sample_values": df[col].dropna().head(5).tolist()  # First 5 non-null values
    }
```

### Example Output for Text Column

```json
{
  "name": "LineDescription",
  "dtype": "object",
  "null_count": 12,
  "non_null_count": 1988,
  "unique_values": 847,
  "sample_values": [
    "PROMO P1 TV 15 SEG (2024) XXXX",
    "MATERIAL POP DISPLAY TIENDA",
    "EVENTO INAUGURACIÓN CDMX",
    "ANUNCIO RADIO 30 SEG CAMPAÑA",
    "DISEÑO GRÁFICO BANNER WEB"
  ]
}
```

### User Decisions at This Stage
1. **Which columns to clean?** (e.g., "LineDescription", "VendorName")
2. **Which rows to process?** (optional filters)
3. **Which refinery version?** (v1=English, v2=Spanish, v3=Enhanced Spanish)
4. **Deduplication strategy?** (preserve_all, content_only, aggressive)

---

## 🧹 Stage 3: Refinery Cleaning Process

### Endpoint in the original system
`POST /api/cleaning/process`

### Request Structure
```json
{
  "upload_id": "abc-123-def-456",
  "refinery_type": "v3",
  "columns_to_clean": ["LineDescription", "VendorName"],
  "filters": {
    "Year": ["2024"],
    "Department": ["Marketing"]
  },
  "deduplication_strategy": "content_only"
}
```

### Step 3.1: Load Original Data

```python
# Load the pickled DataFrame
file_path = f"/tmp/upload_{upload_id}.pkl"
df = pd.read_pickle(file_path)
```

### Step 3.2: Apply User Filters (Optional)

```python
# If user selected specific rows via filters
for col, values in request.filters.items():
    if col in df.columns and values:
        df = df[df[col].isin(values)]

# Example: Only process rows where Year=2024 AND Department=Marketing
```

### Step 3.3: Initialize Refinery Pipeline

```python
# Create refinery with version selected by user
pipeline = RefineryPipeline(refinery_type=request.refinery_type)
# refinery_type = "v1" → English refinery
# refinery_type = "v2" → Spanish refinery (legacy)
# refinery_type = "v3" → Enhanced Spanish refinery (recommended)
```

### Step 3.4: Text Cleaning with Refinery

**This is where the magic happens - "clean" fields are created here.**

```python
# Code: refinery/pipeline.py:29-35
def clean_df(self, df: pd.DataFrame, columns: List[str]) -> pd.DataFrame:
    cleaned_df = df.copy()

    for col in columns:
        if col in cleaned_df.columns:
            # 🔑 CREATE NEW COLUMN WITH "clean" PREFIX
            cleaned_df[f"clean{col}"] = cleaned_df[col].apply(
                lambda x: self.refinery.process(str(x)) if pd.notna(x) else x
            )

    return cleaned_df
```

### What the Refinery Does

**Input:** Original messy text
```
"PROMO P1 TV 15 SEG (2024) XXXX MOJIBAKE ¿¿¿"
```

**Refinery Processing (v3 Enhanced Spanish):**
1. **Normalize**: Convert to lowercase
2. **Remove Period Codes**: Strip "P1", "P2", "P3", etc.
3. **Remove Parentheses**: Remove content in `(...)`
4. **Remove Mojibake**: Clean corrupted characters like "XXXX", "¿¿¿"
5. **Remove Special Characters**: Clean punctuation and symbols
6. **Remove Stopwords**: Remove filler words
7. **Preserve Business Terms**: Keep important terms like "TV", "POP", "MEDIOS"
8. **Normalize Whitespace**: Single spaces, trim edges

**Output:** Clean text
```
"promo tv 15 seg"
```

### Column Naming Convention

```python
# Original columns remain untouched
df["LineDescription"]  # Original: "PROMO P1 TV 15 SEG (2024)"
df["VendorName"]       # Original: "TELEVISA S.A. de C.V."

# New "clean" columns are added
df["cleanLineDescription"]  # Clean: "promo tv 15 seg"
df["cleanVendorName"]       # Clean: "televisa"
```

### Important: Original Data is PRESERVED

```python
cleaned_df = df.copy()  # Always work on a copy
# Original columns: LineDescription, VendorName, Amount, Date, etc.
# New columns added: cleanLineDescription, cleanVendorName
```

**Result DataFrame:**
```
| LineDescription              | VendorName          | cleanLineDescription | cleanVendorName |
|------------------------------|---------------------|----------------------|-----------------|
| PROMO P1 TV 15 SEG (2024)    | TELEVISA S.A.       | promo tv 15 seg      | televisa        |
| MATERIAL POP DISPLAY         | IMPRESIONES MX      | material pop display | impresiones mx  |
```

---

## 🔄 Stage 4: Deduplication Process

### Two-Level Deduplication System

The system has **TWO separate deduplication mechanisms** that can work together:

#### Level 1: Pipeline Internal Deduplication
**Location:** `refinery/pipeline.py:94-168`

```python
# In process_full() method
if deduplication_strategy == "preserve_all":
    # ✅ NO DEDUPLICATION - Keep all rows
    deduplicated_df = cleaned_df
    duplicates_removed = 0

elif deduplication_strategy == "content_only":
    # ⚡ SMART DEDUPLICATION - Remove duplicates based only on clean columns
    clean_columns = ["cleanLineDescription", "cleanVendorName"]
    deduplicated_df = cleaned_df.drop_duplicates(subset=clean_columns)
    duplicates_removed = len(cleaned_df) - len(deduplicated_df)

elif deduplication_strategy == "aggressive":
    # 🔥 AGGRESSIVE DEDUPLICATION - Remove rows identical across ALL columns
    deduplicated_df = cleaned_df.drop_duplicates()
    duplicates_removed = len(cleaned_df) - len(deduplicated_df)
```

#### Level 2: Universal Deduplication Service
**Location:** `services/cleaning/deduplication.py`

```python
# In cleaning.py:124-136
if deduplication_strategy != "preserve_all":
    clean_columns = [f"clean{col}" for col in request.columns_to_clean]

    deduplicated_df, dedup_stats = universal_deduplication_service.deduplicate(
        cleaned_df,
        key_columns=clean_columns,
        strategy="first"  # Keep first occurrence
    )
```

### Deduplication Strategy Comparison

| Strategy | What It Does | Use Case |
|----------|-------------|----------|
| **preserve_all** | Keeps all rows, no deduplication | Default - preserves legitimate duplicate transactions |
| **content_only** | Removes rows with identical clean text | Reduce LLM cost - same text gets same classification |
| **aggressive** | Removes rows identical across ALL fields | Data cleaning - remove exact duplicates |

### Example: content_only Deduplication

**Before Deduplication:**
```
| LineDescription           | Amount | Date       | cleanLineDescription |
|---------------------------|--------|------------|----------------------|
| PROMO TV 15 SEG P1        | 1500   | 2024-01-10 | promo tv 15 seg      |
| PROMO TV 15 SEG (P1)      | 1500   | 2024-01-10 | promo tv 15 seg      | ← Duplicate (same clean)
| PROMO TV 15 SEG P2        | 1800   | 2024-02-15 | promo tv 15 seg      | ← Duplicate (same clean)
| MATERIAL POP              | 500    | 2024-01-12 | material pop         |
```

**After Deduplication (content_only):**
```
| LineDescription           | Amount | Date       | cleanLineDescription |
|---------------------------|--------|------------|----------------------|
| PROMO TV 15 SEG P1        | 1500   | 2024-01-10 | promo tv 15 seg      | ✅ Kept (first)
| MATERIAL POP              | 500    | 2024-01-12 | material pop         | ✅ Kept (unique)
```

**Statistics Generated:**
```python
{
    "original_rows": 4,
    "deduplicated_rows": 2,
    "duplicates_removed": 2,
    "efficiency_gain_pct": 50.0,  # 50% reduction
    "unique_content_ratio": 0.5    # 50% unique content
}
```

### Why Two Levels?

1. **Pipeline Deduplication**: Fast, basic deduplication using pandas
2. **Universal Service**: Advanced deduplication with detailed statistics and mapping

Both can be used together or independently based on configuration.

### Content Similarity Analysis

The system also analyzes how much content is similar without removing it:

```python
# Code: refinery/pipeline.py:170-213
def _analyze_content_similarity(self, df: pd.DataFrame, columns: List[str]):
    # For each clean column, analyze duplicate patterns
    for col in columns:
        clean_col = f"clean{col}"
        value_counts = df[clean_col].value_counts()

        similar_groups = sum(1 for count in value_counts if count > 1)
        max_group_size = value_counts.max()
        unique_ratio = len(value_counts) / len(df)
```

**Output:**
```json
{
  "content_similarity": {
    "similar_content_groups": 45,
    "max_group_size": 12,
    "unique_content_ratio": 0.65,
    "column_details": {
      "LineDescription": {
        "similar_groups": 30,
        "max_group_size": 12,
        "unique_ratio": 0.65
      },
      "VendorName": {
        "similar_groups": 15,
        "max_group_size": 8,
        "unique_ratio": 0.80
      }
    }
  }
}
```

This tells us:
- **45 groups** of similar content (same clean text appears multiple times)
- **Largest group** has 12 identical texts
- **65% unique** content (35% is duplicated)

---

## 📄 Stage 5: JSON Generation for LLM

### What Gets Saved

**Code:** `cleaning.py:142-159`

```python
# Generate LLM input JSON with ALL clean columns
llm_input = {"entries": []}

# Get all clean-prefixed columns
clean_columns = [f"clean{col}" for col in request.columns_to_clean
                 if f"clean{col}" in cleaned_df.columns]

# Example: clean_columns = ["cleanLineDescription", "cleanVendorName"]

for _, row in cleaned_df.iterrows():
    entry = {}

    # Include ALL clean columns in each entry
    for col in clean_columns:
        entry[col] = str(row[col]) if pd.notna(row[col]) else ""

    # Skip entries where all clean columns are empty
    if any(entry[col].strip() for col in clean_columns):
        llm_input["entries"].append(entry)

# Save to disk
llm_json_path = f"/tmp/llm_input_{upload_id}.json"
with open(llm_json_path, 'w', encoding='utf-8') as f:
    json.dump(llm_input, f, ensure_ascii=False, indent=2)
```

### Output JSON Structure

```json
{
  "entries": [
    {
      "cleanLineDescription": "promo tv 15 seg",
      "cleanVendorName": "televisa"
    },
    {
      "cleanLineDescription": "material pop display",
      "cleanVendorName": "impresiones mx"
    },
    {
      "cleanLineDescription": "evento inauguracion cdmx",
      "cleanVendorName": "producciones eventos"
    }
  ]
}
```

### Key Characteristics

1. **Only Clean Fields**: Original messy text is NOT included in LLM input
2. **All Clean Columns**: If user selected 3 columns to clean, all 3 clean versions are included
3. **No Empty Entries**: Entries where ALL clean fields are empty are skipped
4. **UTF-8 Encoding**: Handles Spanish characters correctly (á, é, í, ñ, etc.)
5. **Pretty Printed**: indent=2 for human readability

### File Storage

```bash
/tmp/
├── upload_{upload_id}.pkl              # Original data (pandas DataFrame)
├── cleaned_{upload_id}.xlsx            # Cleaned data with original + clean columns
├── llm_input_{upload_id}.json          # Clean data for LLM (THIS FILE)
└── llm_response_{upload_id}.json       # LLM output (created later)
```

---

## 📊 Complete Statistics Generated

At the end of the cleaning pipeline, comprehensive statistics are returned:

```json
{
  "batch_id": "batch-789",
  "session_id": "sess-123",
  "upload_id": "abc-123-def-456",
  "download_path": "/api/cleaning/download/cleaned_abc-123-def-456.xlsx",
  "llm_download_path": "/api/cleaning/download/llm_input_abc-123-def-456.json",
  "total_entries": 1500,
  "clean_columns_generated": ["cleanLineDescription", "cleanVendorName"],
  "message": "Datos limpiados exitosamente. Se generaron 2 columnas clean: cleanLineDescription, cleanVendorName",
  "stats": {
    "original_rows": 2000,
    "cleaned_rows": 2000,
    "deduplicated_rows": 1500,
    "duplicates_removed": 500,
    "total_columns": 8,
    "selected_columns": 2,
    "deduplication_strategy": "content_only",
    "content_similarity": {
      "similar_content_groups": 45,
      "max_group_size": 12,
      "unique_content_ratio": 0.75,
      "column_details": {
        "LineDescription": {
          "similar_groups": 30,
          "max_group_size": 12,
          "unique_ratio": 0.75
        },
        "VendorName": {
          "similar_groups": 15,
          "max_group_size": 8,
          "unique_ratio": 0.85
        }
      }
    },
    "universal_deduplication": {
      "original_rows": 2000,
      "deduplicated_rows": 1500,
      "duplicates_removed": 500,
      "deduplication_columns": ["cleanLineDescription", "cleanVendorName"],
      "deduplication_strategy": "first",
      "efficiency_gain_pct": 25.0,
      "unique_content_ratio": 0.75
    }
  }
}
```

---

## 🔑 Critical Invariants and Business Rules

### 1. Original Data Never Modified
```python
cleaned_df = df.copy()  # Always work on a copy
# Original columns remain untouched in the DataFrame
```

### 2. Clean Column Naming Convention
```python
# Rule: "clean" + OriginalColumnName (preserving case)
"LineDescription" → "cleanLineDescription"
"VendorName" → "cleanVendorName"
"invoice_number" → "cleaninvoice_number"
```

### 3. LLM Input Contains ONLY Clean Fields
```python
# LLM never sees original messy text
# Only receives cleaned, normalized text for classification
{
  "cleanLineDescription": "promo tv 15 seg",  # ✅ Clean
  "LineDescription": "PROMO P1 TV..."          # ❌ Not included
}
```

### 4. Empty Value Handling
```python
# Null/NaN values are converted to empty strings
if pd.notna(value):
    entry[col] = str(value)
else:
    entry[col] = ""  # Never send null to LLM
```

### 5. Deduplication is Pre-LLM
```python
# Deduplication happens BEFORE LLM processing
# Reason: Reduce LLM costs and processing time
# Same cleaned text should get same classification
```

### 6. File Cleanup Strategy
```python
# After processing, cleanup large files to save disk space
# Keep: llm_input_*, llm_response_*, validation_*, refined_*, cleaned_*
# Delete: Large intermediate files
file_manager.cleanup_processed_files(
    upload_id=request.upload_id,
    keep_patterns=['llm_input_', 'llm_response_', 'validation_', 'refined_', 'cleaned_']
)
```

---

## 🎯 Summary: What Reaches the LLM

### Data Transformations Summary

```
ORIGINAL FILE (Excel/CSV/JSON)
    ↓
PANDAS DATAFRAME (all columns preserved)
    ↓
FILTERED DATAFRAME (user filters applied)
    ↓
CLEANED DATAFRAME (+ clean* columns added)
    ↓
DEDUPLICATED DATAFRAME (duplicates removed if configured)
    ↓
JSON FOR LLM (only clean* fields, no empty entries)
```

### What the LLM Receives

**Input Structure:**
```json
{
  "entries": [
    {"cleanLineDescription": "promo tv 15 seg", "cleanVendorName": "televisa"},
    {"cleanLineDescription": "material pop display", "cleanVendorName": "impresiones mx"}
  ]
}
```

**What LLM Does NOT Receive:**
- ❌ Original messy text
- ❌ Other non-cleaned columns (Amount, Date, etc.)
- ❌ Empty entries (all clean fields empty)
- ❌ Duplicate entries (if deduplication enabled)
- ❌ Null/NaN values (converted to empty strings)

**What LLM Will Return (next stage):**
```json
{
  "results": [
    {
      "cleanLineDescription": "promo tv 15 seg",
      "cleanVendorName": "televisa",
      "category": "Publicidad en Medios",
      "reason": "Es una promoción de TV de 15 segundos",
      "score": 0.95
    }
  ]
}
```

---

## 🚀 Next Stage: LLM Processing

The JSON file created at stage 5 is then passed to:

```python
# POST /api/llm/process
{
  "upload_id": "abc-123-def-456",
  "chunk_size": 5,
  "max_retries": 3,
  "prompt": "optional custom prompt"
}
```

The LLM will:
1. Read `/tmp/llm_input_{upload_id}.json`
2. Process entries in chunks (default: 5 entries per API call)
3. Add classification: category, reason, score
4. Save to `/tmp/llm_response_{upload_id}.json`

---

## 📝 Implementation Notes for Agent

### Critical Files to Understand

1. **routers/cleaning.py** (lines 76-194)
    - Main endpoint: `POST /api/cleaning/process`
    - Orchestrates the entire pre-LLM pipeline

2. **refinery/pipeline.py** (lines 29-168)
    - `clean_df()`: Where clean columns are created
    - `process_full()`: Main processing with deduplication

3. **services/cleaning/deduplication.py** (lines 26-118)
    - Universal deduplication service
    - Three strategies: preserve_all, content_only, aggressive

4. **refinery/versions/v3_enhanced_spanish.py**
    - Spanish text cleaning implementation
    - 18+ processing nodes for Mexican business data

### Common Pitfalls

1. **Don't Lose Original Data**: Always work on copies
2. **Case-Sensitive Column Names**: "LineDescription" vs "linedescription"
3. **Handle NaN/Null**: Convert to empty strings, never send null to LLM
4. **UTF-8 Encoding**: Essential for Spanish characters
5. **Memory Management**: Large files need streaming, not full loading

### Testing Checklist

- [ ] Upload various formats (Excel, CSV, JSON, JSONL)
- [ ] Test with Spanish characters (á, é, í, ñ, ü)
- [ ] Test with null values
- [ ] Test deduplication strategies
- [ ] Verify clean column naming
- [ ] Check statistics accuracy
- [ ] Validate JSON structure for LLM
- [ ] Test empty entry filtering

---

## 🔧 Go Migration Considerations

### Recommended Approach

1. **DataFrame Equivalent**: Use `github.com/go-gota/gota/dataframe`
2. **Excel Parsing**: Use `github.com/360EntSecGroup-Skylar/excelize`
3. **CSV Parsing**: Standard library `encoding/csv`
4. **JSON Parsing**: Standard library `encoding/json`
5. **Text Processing**: Implement refinery nodes as pure functions
6. **Concurrency**: Process chunks in parallel with goroutines

### Performance Optimizations

```go
// Process chunks in parallel
func ProcessChunks(entries []Entry, chunkSize int) []CleanedEntry {
    chunks := splitIntoChunks(entries, chunkSize)
    results := make([]CleanedEntry, 0, len(entries))

    var wg sync.WaitGroup
    resultChan := make(chan []CleanedEntry, len(chunks))

    for _, chunk := range chunks {
        wg.Add(1)
        go func(c []Entry) {
            defer wg.Done()
            cleaned := processChunk(c)
            resultChan <- cleaned
        }(chunk)
    }

    go func() {
        wg.Wait()
        close(resultChan)
    }()

    for cleaned := range resultChan {
        results = append(results, cleaned...)
    }

    return results
}
```

### Memory Management

```go
// Stream large files instead of loading everything
func StreamLargeExcel(filename string, processor func([]Row)) error {
    f, err := excelize.OpenReader(filename)
    if err != nil {
        return err
    }
    defer f.Close()

    rows, err := f.Rows("Sheet1")
    if err != nil {
        return err
    }

    batch := make([]Row, 0, 1000)
    for rows.Next() {
        row := rows.Columns()
        batch = append(batch, row)

        if len(batch) >= 1000 {
            processor(batch)
            batch = batch[:0] // Reset slice
        }
    }

    if len(batch) > 0 {
        processor(batch)
    }

    return nil
}
```

---

**End of Pre-LLM Processing Documentation**