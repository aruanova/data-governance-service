# Panel-Datainspector Backend - Complete Context for Go Refactoring

## Executive Summary

Panel-Datainspector is a production data processing system for classifying Mexican business expense data (CFDI line items) using LLM-based classification. The system processes Excel/CSV/JSON files through a pipeline of cleaning, deduplication, LLM classification, and iterative validation refinement. This document captures the complete architecture, business logic, and implementation details needed for a Go refactoring.

## Table of Contents

1. [System Overview](#system-overview)
2. [Architecture](#architecture)
3. [Complete API Specification](#complete-api-specification)
4. [Business Logic & Services](#business-logic--services)
5. [Data Models](#data-models)
6. [Infrastructure Dependencies](#infrastructure-dependencies)
7. [Async Processing Patterns](#async-processing-patterns)
8. [Data Flow & Pipeline](#data-flow--pipeline)
9. [Go Migration Strategy](#go-migration-strategy)

---

## System Overview

### Purpose
Classify Mexican business expenses (CFDI line items) why custom prompts

### Tech Stack (Current Python)
- **Backend**: FastAPI 0.100+
- **Data Processing**: pandas, openpyxl, pyarrow
- **Task Queue**: Celery + Redis
- **Database**: PostgreSQL (async via asyncpg)
- **LLM**: OpenAI GPT-4 (async client)
- **Monitoring**: Weights & Biases (W&B), Structured JSON logs
- **Frontend**: Next.js 15.3.5 + React 19 + TypeScript

### Key Features
1. **Multi-file batch processing** - Process directories/ZIP files with schema validation
2. **Async distributed processing** - Celery workers with LLM chunk parallelization
3. **Iterative refinement** - Manual validation → prompt improvement → re-classification
4. **Change tracking** - Detect automatic LLM improvements vs manual corrections
5. **Session persistence** - Redis + PostgreSQL for recovery after page reloads
6. **Streaming processing** - Handle files larger than available RAM

---

## Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        FRONTEND (Next.js)                        │
│  3-Tab Workflow: Data Cleaning → Pipeline → Validation          │
└──────────────────────────┬──────────────────────────────────────┘
                           │ HTTP/REST + WebSocket
┌──────────────────────────┴──────────────────────────────────────┐
│                    BACKEND (FastAPI)                             │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐   │
│  │  12 Routers    │  │  25+ Services  │  │  Middleware    │   │
│  │  - pipeline    │  │  - llm         │  │  - logging     │   │
│  │  - cleaning    │  │  - batch       │  │  - CORS        │   │
│  │  - validation  │  │  - validation  │  └────────────────┘   │
│  │  - batch       │  │  - metrics     │                        │
│  │  - session     │  │  - evolution   │                        │
│  │  - tasks       │  └────────────────┘                        │
│  └────────────────┘                                             │
└──────────────────────────┬──────────────────────────────────────┘
                           │
┌──────────────────────────┴──────────────────────────────────────┐
│                    INFRASTRUCTURE LAYER                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │    Redis     │  │  PostgreSQL  │  │   Celery     │         │
│  │  - Sessions  │  │  - Batches   │  │  - Workers   │         │
│  │  - State     │  │  - History   │  │  - Beat      │         │
│  │  - Cache     │  └──────────────┘  │  - Flower    │         │
│  └──────────────┘                    └──────────────┘         │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                 ┌─────────┴─────────┐
                 │   External APIs    │
                 │  - OpenAI (LLM)   │
                 │  - W&B (Metrics)  │
                 └───────────────────┘
```

### Service Layer Organization

```
services/
├── llm/
│   ├── classifier.py          # Core LLM classification logic
│   ├── chunk_optimizer.py     # Dynamic chunk size tuning
│   └── gemini_evaluator.py    # Dual LLM validation
├── batch/
│   ├── batch_processor.py     # Single-file batch logic
│   ├── batch_processor_async.py # Celery orchestration
│   ├── batch_state_manager.py # Redis state + PostgreSQL persistence
│   ├── batch_repository.py    # PostgreSQL data layer
│   └── zip_handler.py         # ZIP file extraction
├── cleaning/
│   └── deduplication.py       # Universal deduplication
├── validation/
│   ├── sampling.py            # Representative sampling
│   ├── iteration_tracking.py # Iteration history
│   ├── search.py              # Keyword search
│   └── change_detection.py    # Automatic vs manual changes
├── evolution/
│   └── classification_evolution.py # Timeline analysis
├── metrics/
│   ├── baseline_metrics.py    # Initial metrics capture
│   ├── comparative_metrics.py # Iteration comparison
│   ├── weave_accuracy.py      # Accuracy scoring
│   └── wandb_traces.py        # W&B integration
├── core/
│   ├── excel_streaming.py     # Memory-efficient Excel reading
│   ├── session_manager.py     # Session persistence
│   ├── file_manager.py        # Temporary file cleanup
│   └── data_integrity.py      # Validation checks
└── maintenance/
    └── cleanup_scheduler.py   # Background cleanup tasks
```

### Router Layer (API Endpoints)

```
routers/
├── pipeline.py         # File upload (async Celery)
├── cleaning.py         # Data cleaning + LLM processing
├── validation.py       # Sampling, validation, refinement (15+ endpoints)
├── batch.py            # Multi-file processing (20+ endpoints)
├── batch_async.py      # Async batch orchestration
├── batch_websocket.py  # Real-time progress updates
├── session.py          # Session CRUD
├── dashboard.py        # Batch history dashboard
├── tasks.py            # Celery task monitoring
├── task_websocket.py   # Task progress updates
├── health.py           # Health checks (filesystem, OpenAI, W&B)
└── maintenance.py      # Cleanup triggers
```

---

## Complete API Specification

### 1. File Upload & Processing

#### `POST /api/pipeline/upload`
**Purpose**: Upload file and start async processing
**Request**:
```typescript
FormData {
  file: File (CSV/XLSX/JSON/JSONL)
}
```
**Response**:
```json
{
  "task_id": "uuid",
  "filename": "data.xlsx",
  "file_size": 1234567,
  "status": "processing",
  "message": "File upload started"
}
```
**Celery Task**: `process_upload.apply_async()` → Queue: `file_processing`

#### `GET /api/pipeline/upload/{upload_id}/status`
**Purpose**: Check upload processing status
**Response**:
```json
{
  "upload_id": "uuid",
  "status": "completed",
  "columns": [{"name": "LineDescription", "dtype": "object", "nulls": 0, "values": ["sample1"]}],
  "row_count": 10000
}
```

### 2. Data Cleaning Pipeline

#### `POST /api/cleaning/sample`
**Purpose**: Preview cleaning on 10 random rows
**Request**:
```json
{
  "upload_id": "uuid",
  "refinery_type": "v3", // v1=English, v2=Spanish(old), v3=Spanish(fixed)
  "columns_to_clean": ["LineDescription"],
  "filters": {"Status": ["Active"]},
  "sample_size": 10
}
```
**Response**:
```json
{
  "original_sample": [{"LineDescription": "LIBRO HABIL MENTAL"}],
  "cleaned_sample": [{"cleanLineDescription": "libro habilidad mental"}],
  "stats": {
    "original_rows": 10000,
    "sample_rows": 10,
    "total_columns": 15,
    "selected_columns": 1
  }
}
```

#### `POST /api/cleaning/process`
**Purpose**: Clean full dataset + generate LLM input JSON
**Request**:
```json
{
  "upload_id": "uuid",
  "refinery_type": "v3",
  "columns_to_clean": ["LineDescription"],
  "filters": {},
  "deduplication_strategy": "content_only", // preserve_all | aggressive | content_only
  "session_id": "session_uuid"
}
```
**Response**:
```json
{
  "batch_id": "batch_uuid",
  "session_id": "session_uuid",
  "upload_id": "upload_uuid",
  "download_path": "/api/cleaning/download/cleaned_{upload_id}.xlsx",
  "llm_download_path": "/api/cleaning/download/llm_input_{upload_id}.json",
  "stats": {
    "original_rows": 10000,
    "cleaned_rows": 10000,
    "deduplicated_rows": 8000,
    "duplicates_removed": 2000,
    "deduplication_strategy": "content_only"
  },
  "total_entries": 8000,
  "clean_columns_generated": ["cleanLineDescription"]
}
```

**Business Logic**:
1. Load uploaded DataFrame from `/tmp/upload_{upload_id}.pkl`
2. Apply filters
3. Create `RefineryPipeline` with version (v1/v2/v3)
4. Process columns → create `clean{ColumnName}` columns
5. Apply universal deduplication on clean columns
6. Generate LLM input JSON with structure:
```json
{
  "entries": [
    {
      "cleanLineDescription": "text1",
      "cleanField2": "text2"
    }
  ]
}
```
7. Save to `/tmp/llm_input_{upload_id}.json`

#### `POST /api/llm/process`
**Purpose**: Classify data with LLM
**Request**:
```json
{
  "upload_id": "uuid",
  "prompt": "custom prompt (optional)",
  "sample_size": 0,  // 0 = process all
  "chunk_size": 50,  // items per chunk
  "max_retries": 3
}
```
**Response**:
```json
{
  "message": "Pipeline LLM completado exitosamente",
  "download_url": "/api/cleaning/download/llm_response_{upload_id}.json",
  "llm_response": null, // null for >1000 records
  "processed_entries": 8000,
  "total_records": 8000,
  "preview_records": [...], // first 100
  "file_size_mb": 12.5,
  "processing_stats": {
    "total_processed": 8000,
    "categories_found": 15,
    "average_score": 0.85,
    "with_high_confidence": 6800,
    "ambiguous": 200  // score = -1
  },
  "wandb_metadata": {
    "run_path": "entity/project/run_id",
    "wandb_url": "https://wandb.ai/...",
    "total_processing_time": 120.5,
    "total_tokens_used": 250000,
    "parallel_processing": true,
    "concurrency_limit": 3,
    "baseline_metrics_captured": true
  }
}
```

**Business Logic** (Critical for Go):
1. Load LLM input JSON from `/tmp/llm_input_{upload_id}.json`
2. Detect **ALL** clean fields dynamically (e.g., `cleanLineDescription`, `cleanField2`)
3. Create DataFrame with ALL clean fields + `_row_index` for tracking
4. Split into chunks (default 50 items)
5. **Parallel processing** with semaphore (concurrency limit = 3):
   ```go
   semaphore := make(chan struct{}, concurrency_limit)
   for chunk := range chunks {
       semaphore <- struct{}{}
       go func(c Chunk) {
           defer func() { <-semaphore }()
           processChunk(c)
       }(chunk)
   }
   ```
6. For each chunk:
    - Format prompt with **ALL** clean fields
    - Call OpenAI API (async)
    - Parse JSON response
    - **Validate response count matches input count** (critical!)
    - Normalize field names: `clase/category/classification` → `category`
    - Add `_row_index` to maintain 1:1 relationship
    - Retry on failure (max 3 times with exponential backoff)
7. Merge results using `_row_index` (NOT content-based merge!)
8. **Capture baseline metrics** for comparison
9. Save to `/tmp/llm_response_{upload_id}.json`

**LLM Prompt Structure** (hardcoded, critical for Go):
- System role: "You are an expert accountant at OXXO"
- 24 predefined categories with detailed rules
- Input format: `{"entries": [{"cleanFieldName": "text"}]}`
- Output format: `{"results": [{"cleanFieldName": "text", "category": "Pop", "reason": "...", "score": 0.9}]}`
- **CRITICAL MANDATE**: Output must have same count as input (no skipping!)

### 3. Validation & Refinement

#### `POST /api/validation/iteration/start`
**Purpose**: Start new validation iteration
**Request**:
```json
{
  "upload_id": "uuid",
  "exclude_validated": true
}
```
**Response**:
```json
{
  "iteration_number": 2,
  "message": "Started iteration 2",
  "validated_indices_count": 150
}
```

#### `POST /api/validation/sample`
**Purpose**: Generate validation sample (stratified or confidence-filtered)
**Request**:
```json
{
  "upload_id": "uuid",
  "strategy": "stratified",  // or "diversity"
  "target_sample_size": 100,
  "confidence_threshold": 0.7  // optional: filter low-confidence records
}
```
**Response**:
```json
{
  "sample_entries": [
    {
      "original_index": 123,
      "original_entry": {"cleanLineDescription": "text"},
      "llm_classification": {
        "category": "Pop",
        "reason": "Contains 'vinil' keyword",
        "score": 0.9
      },
      "user_validation": {
        "is_correct": null,
        "corrected_category": null,
        "user_notes": null,
        "validated_at": null
      }
    }
  ],
  "sampling_stats": {
    "total_records": 8000,
    "target_sample_size": 100,
    "actual_sample_size": 95,
    "sampling_percentage": 1.19,
    "categories_represented": 12,
    "category_breakdown": {
      "Pop": {"count": 15, "percentage": 15.8}
    }
  },
  "confidence_stats": { // only if confidence_threshold provided
    "total_records": 8000,
    "filtered_records": 500,
    "filter_percentage": 6.25,
    "confidence_threshold": 0.7,
    "confidence_distribution": {
      "high": 6800, "medium": 700, "low": 300, "ambiguous": 200
    }
  }
}
```

**Sampling Algorithm**:
```python
# Stratified sampling (proportional representation)
for category in categories:
    category_records = records[records.category == category]
    sample_size = int(total_sample * (len(category_records) / total_records))
    samples.extend(category_records.sample(sample_size))

# Confidence-filtered (focus on low-confidence)
low_conf = records[records.score < threshold]
samples = low_conf.sample(min(target_sample, len(low_conf)))
```

#### `POST /api/validation/submit`
**Purpose**: Submit manual validations
**Request**:
```json
{
  "upload_id": "uuid",
  "validations": [
    {
      "original_index": 123,
      "is_correct": false,
      "corrected_category": "Publicidad",
      "user_notes": "Should be Publicidad, not Pop"
    }
  ]
}
```
**Response**:
```json
{
  "message": "15 validations submitted successfully",
  "validation_stats": {
    "total_validated": 15,
    "accuracy_rate": 0.80,
    "categories_validated": ["Pop", "Publicidad"],
    "validation_breakdown": {
      "Pop": {"correct": 8, "incorrect": 2}
    }
  }
}
```

#### `POST /api/validation/refine`
**Purpose**: Generate refined prompt based on validations
**Request**:
```json
{
  "upload_id": "uuid",
  "use_validation_feedback": true,
  "additional_examples": []
}
```
**Response**:
```json
{
  "message": "Refined prompt generated",
  "refined_prompt": "...enhanced prompt with examples...",
  "validation_stats": {...},
  "improvement_metrics": {
    "examples_added": 15,
    "categories_improved": ["Pop", "Publicidad"]
  }
}
```

#### `POST /api/validation/process-refined`
**Purpose**: Re-process with refined prompt + compare metrics
**Request**:
```json
{
  "upload_id": "uuid",
  "refined_prompt": "...enhanced prompt...",
  "iteration_number": 2,
  "chunk_size": 50,
  "max_retries": 3,
  "use_baseline_comparison": true
}
```
**Response**:
```json
{
  "message": "Refined processing completed",
  "llm_response": {...},
  "processed_entries": 8000,
  "iteration_number": 2,
  "comparative_metrics": {
    "iteration_number": 2,
    "timestamp": "2025-01-15T10:30:00",
    "overall_improvement_score": 12.5,
    "performance_comparison": {
      "baseline_processing_time": 120.5,
      "refined_processing_time": 115.2,
      "time_improvement_pct": 4.4
    },
    "quality_comparison": {
      "baseline_accuracy": 0.80,
      "refined_accuracy": 0.90,
      "accuracy_improvement": 0.10,
      "ambiguous_reduction": 50  // reduced from 200 to 100
    },
    "distribution_comparison": {
      "category_shifts": {
        "Pop": {"from": 1000, "to": 950},
        "Publicidad": {"from": 500, "to": 550}
      }
    },
    "recommendations": [
      "Accuracy improved significantly",
      "Consider validating more 'Pop' classifications"
    ]
  }
}
```

#### `POST /api/validation/search`
**Purpose**: Search records by keyword (for targeted validation)
**Request**:
```json
{
  "upload_id": "uuid",
  "keyword": "vinil",
  "fields": ["cleanLineDescription"],  // null = search all Clean* fields
  "category": "Pop",  // optional filter
  "page": 1,
  "limit": 50
}
```
**Response**:
```json
{
  "results": [
    {
      "original_index": 123,
      "matched_fields": ["cleanLineDescription"],
      "highlights": {
        "cleanLineDescription": "instalacion <mark>vinil</mark>"
      },
      "all_clean_fields": {
        "cleanLineDescription": "instalacion vinil"
      },
      "current_classification": {
        "category": "Pop",
        "score": 0.9,
        "reason": "Contains vinil keyword"
      },
      "validation_history": [
        {
          "iteration": 1,
          "llm_category": "Pop",
          "user_validation": {
            "is_correct": true,
            "validated_at": "2025-01-15T10:00:00"
          },
          "change_type": "manual"
        },
        {
          "iteration": 2,
          "llm_category": "Pop",
          "user_validation": null,
          "change_type": "automatic",  // LLM maintained same category
          "old_category": "Publicidad",
          "old_score": 0.6,
          "new_score": 0.9
        }
      ],
      "has_improved": true  // improved from iteration 1 to 2
    }
  ],
  "total_count": 150,
  "page": 1,
  "limit": 50,
  "total_pages": 3,
  "keyword": "vinil",
  "searched_fields": ["cleanLineDescription"],
  "category_filter": "Pop"
}
```

### 4. Batch Processing (Multi-File)

#### `GET /api/batch/available-directories`
**Purpose**: List directories in `data/input/`
**Response**:
```json
{
  "directories": [
    {
      "name": "2024-Q1",
      "file_count": 15,
      "total_size_mb": 250.5
    }
  ]
}
```

#### `POST /api/batch/inspect-directory`
**Purpose**: Scan directory + validate schemas (lazy parsing)
**Request**:
```json
{
  "directory_path": "2024-Q1",  // or "(raíz)" for root
  "file_patterns": ["*.xlsx", "*.csv"]
}
```
**Response**:
```json
{
  "batch_id": "batch_uuid",
  "directory_path": "2024-Q1",
  "files": [
    {
      "file_path": "/data/input/2024-Q1/file1.xlsx",
      "file_name": "file1.xlsx",
      "size_bytes": 1048576,
      "size_mb": 1.0,
      "modified_at": "2025-01-15T10:00:00",
      "extension": ".xlsx"
    }
  ],
  "total_files": 15,
  "total_size_mb": 250.5,
  "schema_validation": {
    "batch_id": "batch_uuid",
    "total_files": 15,
    "files_validated": 15,
    "common_columns": [
      {
        "column_name": "LineDescription",
        "present_in_files": ["file1.xlsx", "file2.xlsx"],
        "missing_in_files": [],
        "presence_count": 15,
        "presence_percentage": 100.0,
        "data_types": {"file1.xlsx": "object", "file2.xlsx": "object"},
        "has_type_conflict": false,
        "unique_types": ["object"]
      }
    ],
    "common_columns_count": 5,
    "optional_columns": [...],
    "has_compatible_schema": true,
    "validation_timestamp": "2025-01-15T10:00:00"
  }
}
```

**Business Logic** (Critical):
1. Scan directory for files matching patterns
2. **Lazy parse** each file (read only first row for columns)
3. **Cache schemas in Redis** with key: `schema:{file_path}:{mtime}`
4. Identify common columns (present in ALL files)
5. Identify optional columns (present in SOME files)
6. Detect type conflicts (same column, different dtypes)
7. Store batch state in Redis + PostgreSQL

#### `POST /api/batch/{batch_id}/process`
**Purpose**: Process all files in batch + consolidate
**Request**:
```json
{
  "selected_columns": ["LineDescription"],
  "refinery_type": "v3",
  "deduplication_strategy": "content_only",
  "chunk_size": 1000,  // streaming chunk size
  "exclude_files": ["file3.xlsx"]
}
```
**Response**:
```json
{
  "batch_id": "batch_uuid",
  "task_id": "celery_task_uuid",
  "status": "processing",
  "message": "Batch processing started",
  "progress_url": "/api/tasks/{task_id}/status"
}
```

**Celery Workflow** (Critical for Go):
```python
# Task chain: process files → consolidate
chain(
    process_batch_files.s(batch_id, files, selected_columns, refinery_type),
    consolidate_files_task.s(batch_id)
).apply_async()
```

**File Processing Algorithm** (Streaming):
```python
# For each file:
for file in files:
    # 1. Stream read in chunks (1000 rows)
    for chunk in pd.read_csv(file, chunksize=1000):
        # 2. Apply cleaning to selected columns
        cleaned_chunk = refinery.clean_df(chunk, selected_columns)

        # 3. Accumulate in temporary parquet (memory-efficient)
        cleaned_chunk.to_parquet(
            f"/tmp/batch_{batch_id}/file_{i}_chunk_{j}.parquet",
            compression="snappy"
        )

    # 4. Merge chunks for this file
    all_chunks = [read parquet chunks]
    file_result = pd.concat(all_chunks)
    file_result.to_parquet(f"/tmp/batch_{batch_id}/file_{i}.parquet")

    # 5. Update progress
    update_batch_state(batch_id, {"files_processed": i+1})

# 6. Consolidation (after all files processed)
all_files = [read all file parquets]
consolidated = pd.concat(all_files)
consolidated["source_file"] = file_names  # track origin

# 7. Cross-file deduplication
deduplicated = universal_dedup(consolidated, clean_columns, strategy)

# 8. Generate LLM input JSON
llm_input = {"entries": deduplicated.to_dict(orient="records")}
save_json(f"/tmp/batch_{batch_id}/llm_input.json", llm_input)
```

#### `GET /api/batch/{batch_id}/results`
**Purpose**: Get consolidated results after completion
**Response**:
```json
{
  "batch_id": "batch_uuid",
  "status": "consolidation_complete",
  "files_processed": 15,
  "consolidated_file_path": "/tmp/batch_{id}/consolidated.parquet",
  "llm_input_path": "/tmp/batch_{id}/llm_input.json",
  "total_rows_before_consolidation": 100000,
  "total_rows_after_consolidation": 100000,
  "total_rows_after_deduplication": 75000,
  "duplicates_removed": 25000,
  "processing_time_seconds": 180,
  "consolidation_time_seconds": 30,
  "clean_columns_generated": ["cleanLineDescription"],
  "selected_columns": ["LineDescription"]
}
```

### 5. Session Management

#### `POST /api/session/create`
**Purpose**: Create new session (for recovery)
**Response**:
```json
{
  "session_id": "session_uuid",
  "created_at": "2025-01-15T10:00:00",
  "expires_at": "2025-01-22T10:00:00",  // 7 days TTL
  "current_step": "initialized",
  "state": {}
}
```

#### `GET /api/session/{session_id}`
**Purpose**: Retrieve session state
**Response**:
```json
{
  "session_id": "session_uuid",
  "created_at": "2025-01-15T10:00:00",
  "last_updated": "2025-01-15T10:30:00",
  "current_step": "batch_progress",
  "state": {
    "batch_id": "batch_uuid",
    "upload_id": "upload_uuid",
    "batch_step": "progress",
    "batch_processing_started": true,
    "selected_columns": ["LineDescription"]
  }
}
```

#### `PUT /api/session/{session_id}/state`
**Purpose**: Update session state
**Request**:
```json
{
  "current_step": "validation",
  "state": {
    "iteration_number": 2,
    "validated_count": 50
  }
}
```

### 6. Task Monitoring

#### `GET /api/tasks/{task_id}/status`
**Purpose**: Get Celery task status
**Response**:
```json
{
  "task_id": "task_uuid",
  "status": "PROGRESS",  // PENDING | PROGRESS | SUCCESS | FAILURE
  "result": {
    "current": 50,
    "total": 100,
    "status": "Processing file 5 of 15"
  },
  "traceback": null
}
```

#### `WS /ws/tasks/{task_id}`
**Purpose**: Real-time task progress updates
**Messages**:
```json
{
  "type": "progress",
  "task_id": "task_uuid",
  "data": {
    "current": 50,
    "total": 100,
    "percentage": 50,
    "status": "Processing..."
  }
}
```

### 7. Health & Maintenance

#### `GET /api/health/all`
**Purpose**: Comprehensive health check
**Response**:
```json
{
  "status": "healthy",
  "timestamp": "2025-01-15T10:00:00",
  "checks": {
    "filesystem": {
      "status": "healthy",
      "tmp_writable": true,
      "disk_space_mb": 50000
    },
    "openai": {
      "status": "healthy",
      "api_key_set": true,
      "test_request_successful": true
    },
    "wandb": {
      "status": "healthy",
      "api_key_set": true,
      "project": "datainspector",
      "entity": "oxxo"
    },
    "redis": {
      "status": "healthy",
      "ping_successful": true
    },
    "postgres": {
      "status": "healthy",
      "connection_successful": true
    }
  }
}
```

---

## Business Logic & Services

### 1. LLM Classifier Service (Most Critical)

**Location**: `services/llm/classifier.py`

**Core Functions**:

```python
async def classify_batch(
    df: pd.DataFrame,
    chunk_size: int = 50,
    max_retries: int = 3,
    custom_prompt: str = None
) -> tuple[pd.DataFrame, dict]:
    """
    Main LLM classification function.

    Args:
        df: DataFrame with clean* columns and _row_index
        chunk_size: Items per chunk (default 50)
        max_retries: Retries per chunk (default 3)
        custom_prompt: Override default prompt

    Returns:
        (result_df, metadata)
        - result_df: Original df + category, reason, score columns
        - metadata: Processing stats, W&B info, tokens used

    Critical invariants:
    1. Output rows MUST equal input rows (1:1 mapping)
    2. Use _row_index for merging (NOT content!)
    3. Normalize field names: clase/category → category
    4. Handle response count mismatches (fill defaults or match by description)
    """

    # 1. Detect clean fields dynamically
    clean_fields = [col for col in df.columns if col.startswith('clean')]

    # 2. Add row index for tracking
    df['_row_index'] = range(len(df))

    # 3. Split into chunks
    chunks = [df.iloc[i:i+chunk_size] for i in range(0, len(df), chunk_size)]

    # 4. Parallel processing with semaphore
    semaphore = asyncio.Semaphore(concurrency_limit)
    tasks = [
        process_chunk(chunk, i, semaphore, client, chunk_size, max_retries, run, custom_prompt, clean_fields)
        for i, chunk in enumerate(chunks)
    ]
    results = await asyncio.gather(*tasks)

    # 5. Validate and merge results
    all_results = []
    for chunk_result in results:
        if not chunk_result["success"]:
            raise ValueError(f"Chunk failed: {chunk_result['error']}")
        all_results.extend(chunk_result["results"])

    # 6. Create results DataFrame
    results_df = pd.DataFrame(all_results)

    # 7. CRITICAL: Merge using _row_index (preserves 1:1 relationship)
    results_df = results_df.set_index('_row_index').sort_index()
    df = df.set_index('_row_index').sort_index()

    result_df = df.copy()
    result_df['category'] = results_df['category']
    result_df['reason'] = results_df['reason']
    result_df['score'] = results_df['score']

    # 8. Remove temporary index
    result_df = result_df.reset_index(drop=True)

    # 9. Capture baseline metrics
    baseline_metrics = baseline_metrics_service.capture_baseline_metrics(df, result_df, metadata)

    return result_df, metadata
```

**Chunk Processing** (Critical):

```python
async def process_chunk(
    chunk: pd.DataFrame,
    chunk_index: int,
    semaphore: asyncio.Semaphore,
    client: openai.AsyncOpenAI,
    chunk_size: int,
    max_retries: int,
    run: wandb.Run,
    custom_prompt: str,
    clean_field_names: list
) -> dict:
    """
    Process a single chunk with retry logic.

    Returns:
        {
            "success": bool,
            "results": [{"cleanField": "text", "category": "Pop", "reason": "...", "score": 0.9, "_row_index": 123}],
            "error": str | None,
            "metadata": {...}
        }

    Critical error handling:
    1. Response count mismatch → match by description or fill with defaults
    2. JSON parse error → retry with exponential backoff
    3. API error → retry up to max_retries
    4. Field name normalization: clase/category/classification → category
    """

    async with semaphore:
        retry_count = 0
        while retry_count < max_retries:
            try:
                # 1. Build payload with ALL clean fields
                payload = {
                    "entries": [
                        {field: str(row[field]) for field in clean_field_names}
                        for _, row in chunk.iterrows()
                    ]
                }

                # 2. Format prompt
                prompt = _format_prompt(payload, custom_prompt, clean_field_names)

                # 3. Call OpenAI API
                response = await client.chat.completions.create(
                    model="gpt-4o-mini",
                    messages=[{"role": "user", "content": prompt}]
                )

                # 4. Parse response
                content = response.choices[0].message.content
                parsed = json.loads(content.strip())
                results = parsed["results"]

                # 5. CRITICAL: Validate count
                if len(results) != len(chunk):
                    results = fix_count_mismatch(results, chunk, clean_field_names[0])

                # 6. Normalize field names
                normalized = [normalize_llm_response(r, chunk_index) for r in results]

                # 7. Add row indices
                for i, result in enumerate(normalized):
                    result['_row_index'] = int(chunk.iloc[i]['_row_index'])

                # 8. Log to W&B
                run.log({
                    "llm/chunk_tokens": response.usage.total_tokens,
                    "llm/chunk_time": time.time() - start_time,
                    "llm/chunk_success": True
                })

                return {"success": True, "results": normalized, "error": None, "metadata": {...}}

            except Exception as e:
                retry_count += 1
                if retry_count >= max_retries:
                    return {"success": False, "results": [], "error": str(e), "metadata": {...}}
                await asyncio.sleep(0.5 * (2 ** retry_count))  # Exponential backoff
```

**Count Mismatch Fix** (Critical):

```python
def fix_count_mismatch(results: list, chunk: pd.DataFrame, primary_field: str) -> list:
    """
    Fix when LLM returns wrong number of results.

    Strategy:
    1. Create mapping: description → result
    2. Match each input row to result
    3. Fill missing with defaults: {"category": "Indeterminado", "score": -1}
    """

    # Create mapping
    result_map = {r[primary_field].strip(): r for r in results}

    # Match inputs
    fixed = []
    for _, row in chunk.iterrows():
        desc = str(row[primary_field]).strip()
        if desc in result_map:
            fixed.append(result_map[desc])
        else:
            # Default for missing
            fixed.append({
                primary_field: desc,
                "category": "Indeterminado",
                "reason": "No classification returned",
                "score": -1
            })

    return fixed
```

**Prompt Formatting** (Hardcoded Categories):

```python
def _format_prompt(data: dict, custom_prompt: str, clean_field_names: list) -> str:
    """
    Format prompt for LLM.

    Default prompt (if custom_prompt is None):
    - Expert accountant at OXXO
    - 24 predefined categories with rules
    - Input format: {"entries": [{"cleanField": "text"}]}
    - Output format: {"results": [{"cleanField": "text", "category": "Pop", "reason": "...", "score": 0.9, "clase": "Pop"}]}
    - CRITICAL MANDATE: Output count must match input count

    Categories (24 total):
    1. Pop - In-store promotional materials
    2. Spots TV - TV advertising
    3. Medios Digitales - Digital/social media ads
    4. Publicidad - General advertising (not in-store)
    5. Spots Radio - Radio advertising
    6. Eventos Especiales - Special events (literal match)
    7. Promoción Cerveza - Beer promotions
    8. Eventos/Festivales - Public events
    9. Promoción Botellas - Liquor promotions
    10. Evento capacitación - Training events
    11. Uniformes - Employee uniforms
    12. RADET - Specific internal code
    13. Eventos en Tienda - In-store events
    14. Evento comunicación - Communication events
    15. Hospedaje - Lodging
    16. Perifoneo - Loudspeaker advertising
    17. Música - Musical performances
    18. Prensa - Print media
    19. Viajes - Travel expenses
    20. Viáticos - Per diem
    21. RAC - Specific internal code
    22. Comida - Food/beverage products
    23. Otros - Operational expenses (fallback)
    24. Indeterminado - Unintelligible (use score -1)

    Classification rules (priority order):
    1. Promoción Cerveza/Botellas > Pop > Publicidad
    2. Specific media > Publicidad
    3. Named categories (RAC/RADET) > descriptive
    4. Specific events > general events

    Score ranges:
    - 0.9-1.0: Exact keyword + clear context
    - 0.7-0.9: Strong semantic match
    - 0.4-0.7: Partial match/inference
    - 0.0-0.4: Weak match
    - -1: Only for Indeterminado
    """

    if custom_prompt:
        return f"{custom_prompt}\n\n# DATA:\n{json.dumps(data)}"

    # Build default prompt with categories and rules...
    return default_prompt + "\n\n# DATA:\n" + json.dumps(data)
```

### 2. Refinery Pipeline (Text Cleaning)

**Location**: `refinery/pipeline.py`

**Versions**:
- `v1` (English): Basic normalization
- `v2` (Spanish - old): Spanish-specific cleaning
- `v3` (Spanish - fixed): Enhanced Spanish cleaning with bug fixes

**Processing Nodes**:

```python
class RefineryPipeline:
    def __init__(self, refinery_type: str = "v3"):
        """
        Initialize refinery with version.

        v1 (English):
        - Lowercase
        - Remove extra spaces
        - Strip whitespace

        v2 (Spanish - deprecated):
        - All v1 features
        - Remove accents
        - Spanish stopwords removal
        - BUG: Incorrect regex patterns

        v3 (Spanish - recommended):
        - All v2 features
        - Fixed regex patterns
        - Better handling of abbreviations
        - Improved stopword list
        """
        self.refinery = refinery_registry.create(refinery_type)

    def clean_df(self, df: pd.DataFrame, columns: list) -> pd.DataFrame:
        """
        Clean DataFrame columns.

        Creates new columns with 'clean' prefix:
        - Input: "LineDescription"
        - Output: "cleanLineDescription"

        Preserves original columns for reference.
        """
        cleaned = df.copy()
        for col in columns:
            cleaned[f"clean{col}"] = df[col].apply(
                lambda x: self.refinery.process(str(x)) if pd.notna(x) else x
            )
        return cleaned

    def process_full(self, df, columns, deduplication_strategy):
        """
        Full processing pipeline.

        1. Clean columns
        2. Apply deduplication strategy:
           - preserve_all: Keep all rows
           - content_only: Deduplicate by clean columns only
           - aggressive: Deduplicate by ALL columns
        3. Return cleaned DataFrame + stats
        """
        cleaned = self.clean_df(df, columns)

        if deduplication_strategy == "content_only":
            clean_cols = [f"clean{c}" for c in columns]
            deduplicated = cleaned.drop_duplicates(subset=clean_cols)
        elif deduplication_strategy == "aggressive":
            deduplicated = cleaned.drop_duplicates()
        else:
            deduplicated = cleaned

        stats = {
            "original_rows": len(df),
            "cleaned_rows": len(cleaned),
            "deduplicated_rows": len(deduplicated),
            "duplicates_removed": len(cleaned) - len(deduplicated)
        }

        return {"cleaned_df": deduplicated, "stats": stats}
```

### 3. Universal Deduplication Service

**Location**: `services/cleaning/deduplication.py`

```python
class UniversalDeduplicationService:
    def deduplicate_by_columns(
        self,
        df: pd.DataFrame,
        columns: list | str,
        strategy: str = "content_only",
        keep: str = "first"
    ) -> dict:
        """
        Universal deduplication.

        Strategies:
        - preserve_all: No deduplication
        - content_only: Deduplicate by specified columns only
        - aggressive: Deduplicate by ALL columns

        Returns:
        {
            "deduplicated_df": DataFrame,
            "stats": {
                "original_rows": int,
                "deduplicated_rows": int,
                "duplicates_removed": int,
                "efficiency_gain_pct": float,
                "unique_content_ratio": float
            },
            "mapping_info": {...}
        }
        """

        if strategy == "preserve_all":
            return {"deduplicated_df": df.copy(), "stats": {...}}

        if strategy == "content_only":
            deduplicated = df.drop_duplicates(subset=columns, keep=keep)
        else:  # aggressive
            deduplicated = df.drop_duplicates(keep=keep)

        stats = {
            "original_rows": len(df),
            "deduplicated_rows": len(deduplicated),
            "duplicates_removed": len(df) - len(deduplicated),
            "efficiency_gain_pct": (len(df) - len(deduplicated)) / len(df) * 100
        }

        return {"deduplicated_df": deduplicated, "stats": stats, "mapping_info": {...}}

    def map_results_back(
        self,
        unique_results: pd.DataFrame,
        original_df: pd.DataFrame,
        mapping_column: str,
        result_columns: list = None
    ) -> pd.DataFrame:
        """
        Map LLM results back to ALL original records (before deduplication).

        Use case: If you deduplicated 10K → 5K for LLM efficiency,
        this maps the 5K results back to all 10K original records.

        Algorithm:
        1. Create mapping: mapping_column value → result columns
        2. Apply mapping to original DataFrame
        """

        # Build mapping dictionary
        mapping = {}
        for col in result_columns:
            mapping[col] = dict(zip(
                unique_results[mapping_column],
                unique_results[col]
            ))

        # Apply to original
        result = original_df.copy()
        for col, col_mapping in mapping.items():
            result[col] = original_df[mapping_column].map(col_mapping)

        return result
```

### 4. Validation Sampling Service

**Location**: `services/validation/sampling.py`

```python
class SamplingService:
    def generate_stratified_sample(
        self,
        llm_results: list,
        target_sample_size: int = 100
    ) -> dict:
        """
        Stratified sampling - proportional representation of categories.

        Algorithm:
        1. Group records by category
        2. For each category:
           sample_size = target * (category_count / total_count)
        3. Randomly sample from each category

        Returns:
        {
            "sample_entries": [ValidationEntry],
            "sampling_stats": {...},
            "sampling_metadata": {"strategy": "stratified"}
        }
        """

        df = pd.DataFrame(llm_results)
        categories = df.groupby('category')

        samples = []
        for category, group in categories:
            n = int(target_sample_size * len(group) / len(df))
            category_samples = group.sample(min(n, len(group)))
            samples.append(category_samples)

        sample_df = pd.concat(samples)

        return {
            "sample_entries": sample_df.to_dict(orient="records"),
            "sampling_stats": {...},
            "sampling_metadata": {"strategy": "stratified"}
        }

    def generate_confidence_filtered_sample(
        self,
        llm_results: list,
        confidence_threshold: float = 0.7,
        target_sample_size: int = 100
    ) -> dict:
        """
        Confidence-filtered sampling - focus on low-confidence records.

        Algorithm:
        1. Filter records with score < threshold
        2. Randomly sample from filtered set
        3. If not enough low-conf records, fill with high-conf

        Returns same structure as stratified_sample + confidence_stats
        """

        df = pd.DataFrame(llm_results)
        low_conf = df[df['score'] < confidence_threshold]

        if len(low_conf) >= target_sample_size:
            sample = low_conf.sample(target_sample_size)
        else:
            sample = pd.concat([
                low_conf,
                df[df['score'] >= confidence_threshold].sample(
                    target_sample_size - len(low_conf)
                )
            ])

        return {
            "sample_entries": sample.to_dict(orient="records"),
            "sampling_stats": {...},
            "confidence_stats": {
                "total_records": len(df),
                "filtered_records": len(low_conf),
                "confidence_threshold": confidence_threshold
            }
        }
```

### 5. Iteration Tracking Service

**Location**: `services/validation/iteration_tracking.py`

```python
class IterationTrackingService:
    def __init__(self):
        self.storage_path = "/tmp/iterations"

    def start_new_iteration(self, upload_id: str) -> int:
        """
        Start new iteration.

        Returns: iteration_number (1-based)

        File structure:
        /tmp/iterations/{upload_id}/
        ├── iteration_1/
        │   ├── validation_sample.json
        │   ├── validations.json
        │   └── refined_prompt.txt
        ├── iteration_2/
        │   └── ...
        └── history.json
        """

        history = self.get_iteration_history(upload_id) or {"iterations": []}
        next_iteration = len(history["iterations"]) + 1

        history["iterations"].append({
            "iteration_number": next_iteration,
            "started_at": datetime.now().isoformat(),
            "status": "in_progress"
        })

        self.save_iteration_history(upload_id, history)
        return next_iteration

    def get_all_validated_indices(self, upload_id: str) -> set:
        """
        Get indices of ALL validated records across ALL iterations.

        Used to exclude from future sampling.
        """

        history = self.get_iteration_history(upload_id)
        if not history:
            return set()

        validated = set()
        for iteration in history["iterations"]:
            iter_num = iteration["iteration_number"]
            validations = self.get_iteration_validations(upload_id, iter_num)
            validated.update([v["original_index"] for v in validations])

        return validated

    def accumulate_validated_examples(self, upload_id: str) -> dict:
        """
        Accumulate ALL validated examples (correct + incorrect).

        Returns:
        {
            "correct": [
                {
                    "original_entry": {...},
                    "llm_category": "Pop",
                    "correct": True,
                    "iteration": 1
                }
            ],
            "incorrect": [
                {
                    "original_entry": {...},
                    "llm_category": "Publicidad",
                    "corrected_category": "Pop",
                    "iteration": 1
                }
            ],
            "total_examples": 150,
            "best_iteration": 2
        }

        Used to build refined prompts with examples.
        """

        history = self.get_iteration_history(upload_id)
        correct = []
        incorrect = []

        for iteration in history["iterations"]:
            validations = self.get_iteration_validations(upload_id, iteration["iteration_number"])

            for val in validations:
                if val["is_correct"]:
                    correct.append({
                        "original_entry": val["original_entry"],
                        "llm_category": val["llm_classification"]["category"],
                        "correct": True,
                        "iteration": iteration["iteration_number"]
                    })
                else:
                    incorrect.append({
                        "original_entry": val["original_entry"],
                        "llm_category": val["llm_classification"]["category"],
                        "corrected_category": val["user_validation"]["corrected_category"],
                        "iteration": iteration["iteration_number"]
                    })

        return {
            "correct": correct,
            "incorrect": incorrect,
            "total_examples": len(correct) + len(incorrect)
        }
```

### 6. Batch State Manager (Redis + PostgreSQL)

**Location**: `services/batch/batch_state_manager.py`

```python
class BatchStateManager:
    def __init__(self):
        self.redis_client = redis.Redis(host="localhost", port=6379, db=1)
        self.db_repository = BatchRepository()  # PostgreSQL
        self.key_prefix = "batch_state:"
        self.ttl = 86400  # 24 hours

    def set_batch_state(self, batch_id: str, state: dict):
        """
        Store batch state in Redis + PostgreSQL.

        Redis: Fast access (24h TTL)
        PostgreSQL: Persistent storage for recovery
        """

        # Store in Redis
        key = f"{self.key_prefix}{batch_id}"
        self.redis_client.setex(key, self.ttl, json.dumps(state))

        # Async persist to PostgreSQL
        asyncio.create_task(self._persist_to_db(batch_id, state))

    def get_batch_state(self, batch_id: str) -> dict | None:
        """
        Get batch state from Redis.

        If not in Redis, try to recover from PostgreSQL.
        """

        # Try Redis first
        key = f"{self.key_prefix}{batch_id}"
        state_json = self.redis_client.get(key)
        if state_json:
            return json.loads(state_json)

        # Try PostgreSQL recovery
        db_state = asyncio.run(self.db_repository.get_batch(batch_id))
        if db_state:
            # Restore to Redis
            self.set_batch_state(batch_id, db_state)
            return db_state

        return None

    async def recover_session_batches(self, session_id: str) -> list:
        """
        Recover all batches for a session.

        Used for session recovery after page reload.
        """

        batches = await self.db_repository.get_session_batches(session_id)

        # Restore active batches to Redis
        for batch in batches:
            if batch["status"] in ["pending", "processing"]:
                self.set_batch_state(batch["batch_id"], batch)

        return batches
```

### 7. Comparative Metrics Service

**Location**: `services/metrics/comparative_metrics.py`

```python
class ComparativeMetricsService:
    def compare_iterations(
        self,
        baseline_metrics: dict,
        refined_metrics: dict,
        iteration_number: int
    ) -> dict:
        """
        Compare baseline vs refined iteration.

        Returns:
        {
            "iteration_number": 2,
            "timestamp": "...",
            "overall_improvement_score": 12.5,  // weighted score
            "performance_comparison": {
                "baseline_processing_time": 120.5,
                "refined_processing_time": 115.2,
                "time_improvement_pct": 4.4
            },
            "quality_comparison": {
                "baseline_accuracy": 0.80,
                "refined_accuracy": 0.90,
                "accuracy_improvement": 0.10,
                "ambiguous_reduction": 50
            },
            "distribution_comparison": {
                "category_shifts": {
                    "Pop": {"from": 1000, "to": 950},
                    "Publicidad": {"from": 500, "to": 550}
                }
            },
            "recommendations": [
                "Accuracy improved significantly",
                "Consider validating more 'Pop' classifications"
            ]
        }

        Accuracy calculation (CRITICAL):
        - Only count records with score != -1 (exclude ambiguous)
        - accuracy = (score >= 0.7) / total_with_valid_score
        - ambiguity_rate = (score == -1) / total
        """

        # Calculate accuracy excluding ambiguous
        baseline_valid = [r for r in baseline_metrics["results"] if r["score"] != -1]
        refined_valid = [r for r in refined_metrics["results"] if r["score"] != -1]

        baseline_accuracy = sum(1 for r in baseline_valid if r["score"] >= 0.7) / len(baseline_valid)
        refined_accuracy = sum(1 for r in refined_valid if r["score"] >= 0.7) / len(refined_valid)

        # Calculate ambiguity reduction
        baseline_ambiguous = sum(1 for r in baseline_metrics["results"] if r["score"] == -1)
        refined_ambiguous = sum(1 for r in refined_metrics["results"] if r["score"] == -1)

        # Generate recommendations based on improvements
        recommendations = []
        if refined_accuracy > baseline_accuracy + 0.05:
            recommendations.append("Accuracy improved significantly")
        if refined_ambiguous < baseline_ambiguous:
            recommendations.append(f"Ambiguity reduced by {baseline_ambiguous - refined_ambiguous} records")

        return {
            "iteration_number": iteration_number,
            "overall_improvement_score": calculate_weighted_score(...),
            "quality_comparison": {
                "baseline_accuracy": baseline_accuracy,
                "refined_accuracy": refined_accuracy,
                "accuracy_improvement": refined_accuracy - baseline_accuracy,
                "ambiguous_reduction": baseline_ambiguous - refined_ambiguous
            },
            "recommendations": recommendations
        }
```

---

## Data Models

### Pydantic Schemas (`models/schemas.py`)

**Core Upload Models**:
```python
class ColumnInfo(BaseModel):
    name: str
    dtype: str  # "object", "int64", "float64"
    nulls: int
    values: List[str]  # Sample values for preview

class UploadResponse(BaseModel):
    upload_id: str
    columns: List[ColumnInfo]

class CleaningConfig(BaseModel):
    upload_id: str
    refinery_type: str  # "v1" | "v2" | "v3"
    columns_to_clean: List[str]
    filters: Optional[Dict[str, List[str]]] = None
    deduplication_strategy: Optional[str] = "content_only"
```

**LLM Processing Models**:
```python
class LLMPipelineConfig(BaseModel):
    upload_id: str
    prompt: str
    sample_size: int
    chunk_size: Optional[int] = None
    max_retries: Optional[int] = None

class LLMPipelineResult(BaseModel):
    message: str
    download_url: Optional[str]
    llm_response: Optional[Dict[str, Any]]  # None for large datasets
    processed_entries: int
    wandb_metadata: Optional[Dict[str, Any]]
    total_records: int
    preview_records: List[Dict[str, Any]]  # First 100
    file_size_mb: float
    processing_stats: Dict[str, Any]
```

**Validation Models**:
```python
class UserValidation(BaseModel):
    is_correct: Optional[bool] = None
    corrected_category: Optional[str] = None
    user_notes: Optional[str] = None
    validated_at: Optional[datetime] = None

class LLMClassification(BaseModel):
    category: str
    reason: str
    score: Optional[float] = None  # -1 for ambiguous

class ValidationEntry(BaseModel):
    original_index: int
    original_entry: Dict[str, Any]
    llm_classification: LLMClassification
    user_validation: UserValidation

class SamplingRequest(BaseModel):
    upload_id: str
    strategy: str = "stratified"
    target_sample_size: Optional[int] = None
    confidence_threshold: Optional[float] = None
```

**Batch Processing Models**:
```python
class FileMetadata(BaseModel):
    file_path: str
    file_name: str
    size_bytes: int
    size_mb: float
    modified_at: str
    extension: str

class ColumnCompatibility(BaseModel):
    column_name: str
    present_in_files: List[str]
    missing_in_files: List[str]
    presence_count: int
    presence_percentage: float
    data_types: Dict[str, str]
    has_type_conflict: bool
    unique_types: List[str]

class SchemaValidationReport(BaseModel):
    batch_id: str
    total_files: int
    files_validated: int
    common_columns: List[ColumnCompatibility]
    common_columns_count: int
    optional_columns: List[ColumnCompatibility]
    has_compatible_schema: bool

class BatchProcessRequest(BaseModel):
    selected_columns: List[str]
    refinery_type: str
    deduplication_strategy: str = "preserve_all"
    chunk_size: Optional[int] = None
    exclude_files: Optional[List[str]] = None

class BatchProcessResult(BaseModel):
    batch_id: str
    status: str
    consolidated_file_path: Optional[str]
    total_records_processed: int
    total_records_after_deduplication: int
    processing_time_seconds: float
```

**Iteration Tracking Models**:
```python
class IterationMetrics(BaseModel):
    iteration_number: int
    accuracy: float
    total_validated: int
    completed_at: Optional[str]
    improvement: Optional[float]

class IterationComparison(BaseModel):
    iterations: List[IterationMetrics]
    summary: IterationSummary

class AccumulatedExamples(BaseModel):
    correct: List[Dict[str, Any]]
    incorrect: List[Dict[str, Any]]
    total_examples: int
    best_iteration: Optional[int]
    overall_trend: Optional[str]  # "improving" | "degrading" | "stable"
```

**Search & Evolution Models**:
```python
class SearchValidationHistory(BaseModel):
    iteration: int
    llm_category: str
    user_validation: Optional[Dict[str, Any]] = None
    validated_at: Optional[str] = None
    change_type: Optional[str] = None  # "manual" | "automatic"
    old_category: Optional[str] = None  # For automatic changes
    old_score: Optional[float] = None
    new_score: Optional[float] = None

class SearchResultItem(BaseModel):
    original_index: int
    matched_fields: List[str]
    highlights: Dict[str, str]
    all_clean_fields: Dict[str, str]
    current_classification: Dict[str, Any]
    validation_history: List[SearchValidationHistory]
    has_improved: Optional[bool] = None
```

### PostgreSQL Schema

**Batch Pipelines Table**:
```sql
CREATE TABLE batch_pipelines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id VARCHAR(100) UNIQUE NOT NULL,
    session_id VARCHAR(100),
    upload_id VARCHAR(100),
    tenant VARCHAR(50) DEFAULT 'test_local',

    -- Status tracking
    status VARCHAR(50) NOT NULL,  -- initialized | schema_validated | processing | completed | failed
    current_phase VARCHAR(100),
    overall_progress INT DEFAULT 0,

    -- Configuration
    config JSONB,  -- Processing configuration

    -- Metrics
    total_files INT,
    files_processed INT,
    total_rows INT,
    processing_total INT,
    processing_completed INT,

    -- File paths
    consolidated_file VARCHAR(500),
    llm_input_file VARCHAR(500),

    -- Timestamps
    created_at TIMESTAMP DEFAULT NOW(),
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    last_updated TIMESTAMP DEFAULT NOW(),

    -- State snapshots (full Redis state)
    state_snapshot JSONB,

    -- Indexes
    INDEX idx_batch_id (batch_id),
    INDEX idx_session_id (session_id),
    INDEX idx_upload_id (upload_id),
    INDEX idx_status (status),
    INDEX idx_created_at (created_at DESC)
);
```

---

## Infrastructure Dependencies

### 1. Redis

**Usage**:
- **DB 0**: Celery broker + result backend
- **DB 1**: Batch state + session management + schema cache

**Key Patterns**:
```
batch_state:{batch_id} → JSON (TTL: 24h)
session:{session_id} → JSON (TTL: 7 days)
schema:{file_path}:{mtime} → JSON (TTL: 1h)
```

**Configuration**:
```python
redis_client = redis.Redis(
    host=os.getenv("REDIS_HOST", "localhost"),
    port=int(os.getenv("REDIS_PORT", 6379)),
    db=1,
    decode_responses=True
)
```

### 2. PostgreSQL

**Connection**:
```python
DATABASE_URL = os.getenv(
    "DATABASE_URL",
    "postgresql+asyncpg://admin:changeme123@localhost:5432/datainspector"
)

# Async engine
engine = create_async_engine(
    DATABASE_URL,
    echo=False,
    poolclass=NullPool,
    future=True
)
```

**Tables**:
- `batch_pipelines`: Batch metadata + state snapshots
- Future: `sessions`, `uploads`, `metrics`

### 3. Celery Configuration

**`celery_app.py`**:
```python
celery_app = Celery(
    'datainspector',
    broker='redis://localhost:6379/0',
    backend='redis://localhost:6379/0',
    include=[
        'app.tasks.file_upload',
        'app.tasks.file_processing',
        'app.tasks.schema_validation',
        'app.tasks.llm_processing',
        'app.tasks.consolidation',
        'app.tasks.maintenance'
    ]
)

celery_app.conf.update(
    task_serializer='json',
    result_expires=3600 * 24,  # 24h
    result_persistent=True,
    result_compression='gzip',

    # Worker settings
    worker_prefetch_multiplier=1,
    worker_max_tasks_per_child=100,

    # Task settings
    task_acks_late=True,
    task_track_started=True,
    task_time_limit=3600,  # 1h hard limit
    task_soft_time_limit=3300,  # 55min soft limit

    # Queues
    task_default_queue='default',
    task_queues=(
        Queue('default', Exchange('default'), routing_key='default'),
        Queue('file_processing', Exchange('file_processing'), routing_key='file_processing'),
        Queue('llm_processing', Exchange('llm_processing'), routing_key='llm_processing'),
        Queue('llm_chunks', Exchange('llm_chunks'), routing_key='llm_chunks'),
        Queue('high_priority', Exchange('high_priority'), routing_key='high_priority')
    ),

    # Task routing
    task_routes={
        'app.tasks.file_processing.*': {'queue': 'file_processing'},
        'app.tasks.llm_processing.*': {'queue': 'llm_processing'},
        'app.tasks.llm_distributed.process_chunk_task': {'queue': 'llm_chunks'},
        'app.tasks.schema_validation.*': {'queue': 'high_priority'}
    }
)
```

**Docker Compose Workers**:
```yaml
# General worker (file processing, orchestration)
celery_worker:
  command: celery -A app.celery_app worker --loglevel=info --concurrency=4 --pool=prefork -n worker@%h
  resources:
    limits:
      cpus: "2.0"
      memory: 4G

# LLM workers (distributed chunk processing)
celery_worker_llm:
  command: celery -A app.celery_app worker --loglevel=info --concurrency=2 --pool=prefork -Q llm_chunks -n llm@%h --prefetch-multiplier=1
  deploy:
    replicas: 2  # Scale based on load
  resources:
    limits:
      cpus: "1.0"
      memory: 2G

# Beat (scheduled tasks)
celery_beat:
  command: celery -A app.celery_app beat --loglevel=info
```

### 4. OpenAI API

**Configuration**:
```python
OPENAI_API_KEY = os.getenv("OPENAI_API_KEY")
MODEL = os.getenv("LLM_MODEL", "gpt-4o-mini")
LLM_CHUNK_SIZE = int(os.getenv("LLM_CHUNK_SIZE", 50))
LLM_MAX_RETRIES = int(os.getenv("LLM_MAX_RETRIES", 3))
LLM_CONCURRENCY_LIMIT = int(os.getenv("LLM_CONCURRENCY_LIMIT", 3))

client = openai.AsyncOpenAI(api_key=OPENAI_API_KEY)
```

**Rate Limits** (Tier 5):
- TPM (Tokens Per Minute): 150,000,000
- RPM (Requests Per Minute): 30,000

**Token Counting**:
```python
import tiktoken

encoding = tiktoken.encoding_for_model(MODEL)

def count_tokens(text: str) -> int:
    return len(encoding.encode(text))

def count_message_tokens(messages: list) -> int:
    tokens = 0
    for msg in messages:
        tokens += 3  # Message overhead
        tokens += count_tokens(msg["content"])
    tokens += 3  # Reply priming
    return tokens
```

### 5. Weights & Biases (W&B)

**Configuration**:
```python
WANDB_API_KEY = os.getenv("WANDB_API_KEY")
WANDB_PROJECT = os.getenv("WANDB_PROJECT", "datainspector")
WANDB_ENTITY = os.getenv("WANDB_ENTITY")

run = wandb.init(
    project=WANDB_PROJECT,
    entity=WANDB_ENTITY,
    config={
        "model": MODEL,
        "chunk_size": chunk_size,
        "concurrency_limit": concurrency_limit
    }
)
```

**Logged Metrics**:
```python
# Per-chunk metrics
run.log({
    "llm/chunk_size": 50,
    "llm/chunk_tokens_used": 5000,
    "llm/chunk_processing_time": 2.5,
    "llm/chunk_success": True,
    "llm/throughput_items_per_sec": 20
})

# Overall metrics
run.log({
    "llm/total_processing_time": 120.5,
    "llm/total_tokens_used": 250000,
    "llm/overall_throughput": 66.4,
    "llm/chunk_success_rate": 0.98,
    "llm/parallel_processing": True
})

# Baseline metrics
run.log({
    "baseline/accuracy": 0.80,
    "baseline/ambiguous_rate": 0.025,
    "baseline/category_distribution": {...}
})
```

---

## Async Processing Patterns

### 1. Celery Task Definitions

**File Upload Task** (`tasks/file_upload.py`):
```python
@celery_app.task(bind=True)
def process_upload(self, temp_path: str, filename: str, file_size: int):
    """
    Process uploaded file.

    1. Load file into DataFrame
    2. Inspect columns (name, dtype, nulls, sample values)
    3. Generate upload_id
    4. Save DataFrame to /tmp/upload_{id}.pkl
    5. Return upload metadata
    """

    try:
        # Load file
        if filename.endswith('.xlsx'):
            df = pd.read_excel(temp_path)
        elif filename.endswith('.csv'):
            df = pd.read_csv(temp_path)
        elif filename.endswith(('.json', '.jsonl')):
            df = pd.read_json(temp_path, lines=filename.endswith('.jsonl'))

        # Generate upload_id
        upload_id = str(uuid.uuid4())

        # Inspect columns
        columns = []
        for col in df.columns:
            columns.append({
                "name": col,
                "dtype": str(df[col].dtype),
                "nulls": int(df[col].isna().sum()),
                "values": df[col].dropna().head(5).astype(str).tolist()
            })

        # Save DataFrame
        df.to_pickle(f"/tmp/upload_{upload_id}.pkl")

        # Clean up temp file
        os.remove(temp_path)

        return {
            "upload_id": upload_id,
            "columns": columns,
            "row_count": len(df)
        }

    except Exception as e:
        raise self.retry(exc=e, countdown=5, max_retries=3)
```

**Schema Validation Task** (`tasks/schema_validation.py`):
```python
@celery_app.task(bind=True)
def validate_batch_schemas(self, batch_id: str, files: list):
    """
    Validate schemas in parallel with caching.

    1. For each file, check Redis cache
    2. If not cached, lazy parse (nrows=1)
    3. Store schema in Redis
    4. Identify common/optional columns
    5. Detect type conflicts
    """

    redis_client = redis.Redis(host="localhost", port=6379, db=1)

    file_schemas = {}
    for file in files:
        # Generate cache key
        cache_key = f"schema:{file['file_path']}:{file['modified_at']}"

        # Check cache
        cached_schema = redis_client.get(cache_key)
        if cached_schema:
            file_schemas[file['file_name']] = json.loads(cached_schema)
            continue

        # Lazy parse
        if file['extension'] == '.xlsx':
            df_sample = pd.read_excel(file['file_path'], nrows=1)
        else:
            df_sample = pd.read_csv(file['file_path'], nrows=1)

        schema = {
            "columns": df_sample.columns.tolist(),
            "dtypes": {col: str(dtype) for col, dtype in df_sample.dtypes.items()}
        }

        # Cache for 1 hour
        redis_client.setex(cache_key, 3600, json.dumps(schema))

        file_schemas[file['file_name']] = schema

    # Identify common columns
    all_columns = set.intersection(*[set(s["columns"]) for s in file_schemas.values()])

    common_columns = []
    for col in all_columns:
        # Check data types
        dtypes = {fname: schema["dtypes"][col] for fname, schema in file_schemas.items() if col in schema["columns"]}
        unique_types = list(set(dtypes.values()))

        common_columns.append({
            "column_name": col,
            "present_in_files": list(dtypes.keys()),
            "data_types": dtypes,
            "has_type_conflict": len(unique_types) > 1,
            "unique_types": unique_types
        })

    return {
        "batch_id": batch_id,
        "common_columns": common_columns,
        "has_compatible_schema": all(not c["has_type_conflict"] for c in common_columns)
    }
```

**File Processing Task** (`tasks/file_processing.py`):
```python
@celery_app.task(bind=True)
def process_batch_files(self, batch_id: str, files: list, selected_columns: list, refinery_type: str):
    """
    Process all files with streaming.

    1. For each file:
       a. Stream read in chunks (1000 rows)
       b. Apply refinery cleaning
       c. Save chunks to temporary parquet
       d. Merge chunks for file
    2. Update progress
    """

    from services.batch.batch_state_manager import batch_state_manager
    from refinery.pipeline import RefineryPipeline

    pipeline = RefineryPipeline(refinery_type)

    for i, file in enumerate(files):
        # Update progress
        batch_state_manager.update_batch_state(batch_id, {
            "current_file": file["file_name"],
            "files_processed": i
        })

        # Stream processing
        chunks = []
        if file["extension"] == ".xlsx":
            # Excel streaming
            for chunk in stream_excel(file["file_path"], chunksize=1000):
                cleaned = pipeline.clean_df(chunk, selected_columns)
                chunks.append(cleaned)
        else:
            # CSV native streaming
            for chunk in pd.read_csv(file["file_path"], chunksize=1000):
                cleaned = pipeline.clean_df(chunk, selected_columns)
                chunks.append(cleaned)

        # Merge chunks
        file_result = pd.concat(chunks, ignore_index=True)
        file_result["source_file"] = file["file_name"]

        # Save to parquet
        output_path = f"/tmp/batch_{batch_id}/file_{i}.parquet"
        file_result.to_parquet(output_path, compression="snappy")

        # Update progress
        batch_state_manager.update_batch_state(batch_id, {
            "files_processed": i + 1
        })

    return {"batch_id": batch_id, "files_processed": len(files)}
```

**Consolidation Task** (`tasks/consolidation.py`):
```python
@celery_app.task(bind=True)
def consolidate_files_task(self, file_processing_result: dict, batch_id: str):
    """
    Consolidate all processed files.

    1. Read all file parquets
    2. Concatenate with PyArrow (memory-efficient)
    3. Apply cross-file deduplication
    4. Generate LLM input JSON
    5. Save consolidated parquet
    """

    from services.cleaning.deduplication import universal_deduplication_service
    import pyarrow.parquet as pq

    # Read all file parquets
    file_paths = glob.glob(f"/tmp/batch_{batch_id}/file_*.parquet")

    # Use PyArrow for efficient concatenation
    tables = [pq.read_table(path) for path in file_paths]
    consolidated_table = pyarrow.concat_tables(tables)
    consolidated = consolidated_table.to_pandas()

    # Cross-file deduplication
    clean_columns = [col for col in consolidated.columns if col.startswith('clean')]
    dedup_result = universal_deduplication_service.deduplicate_by_columns(
        consolidated,
        columns=clean_columns,
        strategy="content_only"
    )

    deduplicated = dedup_result["deduplicated_df"]

    # Generate LLM input JSON
    llm_input = {"entries": deduplicated[clean_columns].to_dict(orient="records")}
    llm_input_path = f"/tmp/batch_{batch_id}/llm_input.json"
    with open(llm_input_path, 'w') as f:
        json.dump(llm_input, f)

    # Save consolidated parquet
    consolidated_path = f"/tmp/batch_{batch_id}/consolidated.parquet"
    deduplicated.to_parquet(consolidated_path, compression="snappy")

    # Update batch state
    batch_state_manager.update_batch_state(batch_id, {
        "status": "consolidation_complete",
        "consolidation": {
            "output_path": consolidated_path,
            "llm_input_path": llm_input_path,
            "total_rows_before": len(consolidated),
            "total_rows_after": len(deduplicated),
            "duplicates_removed": len(consolidated) - len(deduplicated)
        }
    })

    return {"batch_id": batch_id, "status": "consolidation_complete"}
```

**LLM Processing Task** (`tasks/llm_processing.py`):
```python
@celery_app.task(bind=True)
def process_with_llm(self, upload_id: str, json_path: str, chunk_size: int = 50, max_retries: int = 3, custom_prompt: str = None):
    """
    Process LLM classification (calls async classifier).

    1. Load JSON input
    2. Convert to DataFrame
    3. Call async classify_batch()
    4. Save results
    """

    from services.llm.classifier import classify_batch

    # Load input
    with open(json_path, 'r') as f:
        data = json.load(f)

    # Convert to DataFrame
    entries = data["entries"]
    df = pd.DataFrame(entries)

    # Add row index
    df["_original_index"] = range(len(df))

    # Call async classifier
    result_df, metadata = asyncio.run(
        classify_batch(df, chunk_size, max_retries, custom_prompt)
    )

    # Save results
    results = result_df.to_dict(orient="records")
    result_path = f"/tmp/llm_response_{upload_id}.json"
    with open(result_path, 'w') as f:
        json.dump({"results": results}, f)

    return {
        "results": results,
        "metadata": metadata
    }
```

### 2. WebSocket Progress Updates

**WebSocket Handler** (`routers/batch_websocket.py`):
```python
from fastapi import WebSocket, WebSocketDisconnect
from services.batch.batch_state_manager import batch_state_manager

@router.websocket("/ws/batch/{batch_id}")
async def batch_websocket(websocket: WebSocket, batch_id: str):
    """
    Real-time batch progress updates.

    Client connects → Send updates every 500ms → Client disconnects
    """

    await websocket.accept()

    try:
        while True:
            # Get current state
            state = batch_state_manager.get_batch_state(batch_id)

            if state:
                # Send progress update
                await websocket.send_json({
                    "type": "progress",
                    "batch_id": batch_id,
                    "data": {
                        "status": state["status"],
                        "files_processed": state.get("files_processed", 0),
                        "files_total": state.get("total_files", 0),
                        "current_file": state.get("current_file"),
                        "overall_progress": state.get("overall_progress", 0)
                    }
                })

                # Stop if completed/failed
                if state["status"] in ["completed", "failed"]:
                    break

            # Wait before next update
            await asyncio.sleep(0.5)

    except WebSocketDisconnect:
        pass
    finally:
        await websocket.close()
```

### 3. Task Monitoring

**Task Status Endpoint** (`routers/tasks.py`):
```python
@router.get("/tasks/{task_id}/status")
async def get_task_status(task_id: str):
    """
    Get Celery task status.
    """

    from celery.result import AsyncResult

    task = AsyncResult(task_id, app=celery_app)

    if task.state == "PENDING":
        return {"task_id": task_id, "status": "PENDING", "result": None}

    elif task.state == "PROGRESS":
        return {
            "task_id": task_id,
            "status": "PROGRESS",
            "result": task.info  # Custom progress info
        }

    elif task.state == "SUCCESS":
        return {
            "task_id": task_id,
            "status": "SUCCESS",
            "result": task.result
        }

    elif task.state == "FAILURE":
        return {
            "task_id": task_id,
            "status": "FAILURE",
            "result": None,
            "error": str(task.info)
        }
```

---

## Data Flow & Pipeline

### Complete Single-File Flow

```
1. UPLOAD
   POST /api/pipeline/upload
   ↓
   Celery: process_upload.apply_async()
   ↓
   Load file → Inspect columns → Save /tmp/upload_{id}.pkl
   ↓
   Return upload_id + columns

2. CLEANING PREVIEW
   POST /api/cleaning/sample
   ↓
   Load DataFrame → Sample 10 rows → Apply refinery → Return preview

3. FULL CLEANING
   POST /api/cleaning/process
   ↓
   Load DataFrame → Apply filters → Clean columns (create clean*)
   ↓
   Universal deduplication (content_only strategy)
   ↓
   Generate LLM input JSON: {"entries": [{"cleanField": "text"}]}
   ↓
   Save /tmp/llm_input_{id}.json

4. LLM CLASSIFICATION
   POST /api/llm/process
   ↓
   Celery: process_with_llm.apply_async()
   ↓
   Load JSON → Convert to DataFrame → Add _row_index
   ↓
   Split into chunks (50 items each)
   ↓
   Parallel processing with semaphore (concurrency=3):
     For each chunk:
       - Format prompt with ALL clean fields
       - Call OpenAI API (async)
       - Parse JSON response
       - Validate count (fix mismatches)
       - Normalize field names
       - Add _row_index
       - Retry on failure (max 3)
   ↓
   Merge results using _row_index (NOT content!)
   ↓
   Capture baseline metrics (W&B)
   ↓
   Save /tmp/llm_response_{id}.json
   ↓
   Return results + metadata

5. VALIDATION SAMPLING
   POST /api/validation/sample
   ↓
   Load LLM results → Apply sampling strategy (stratified/confidence)
   ↓
   Exclude previously validated indices
   ↓
   Return sample for manual validation

6. MANUAL VALIDATION
   POST /api/validation/submit
   ↓
   Save validations to iteration folder
   ↓
   Calculate accuracy stats
   ↓
   Return validation summary

7. PROMPT REFINEMENT
   POST /api/validation/refine
   ↓
   Load all validated examples (correct + incorrect)
   ↓
   Inject examples into prompt
   ↓
   Return refined prompt

8. RE-CLASSIFICATION
   POST /api/validation/process-refined
   ↓
   Re-run LLM classification with refined prompt
   ↓
   Capture refined metrics
   ↓
   Compare with baseline (comparative_metrics_service)
   ↓
   Return improvement metrics + recommendations

9. ITERATION (Loop back to step 5)
```

### Complete Multi-File (Batch) Flow

```
1. DIRECTORY INSPECTION
   POST /api/batch/inspect-directory
   ↓
   Scan directory for files (*.xlsx, *.csv)
   ↓
   Celery: validate_batch_schemas.apply_async()
   ↓
   For each file (parallel):
     - Check Redis cache for schema
     - If not cached, lazy parse (nrows=1)
     - Extract columns + dtypes
     - Cache in Redis (1h TTL)
   ↓
   Identify common columns (present in ALL files)
   ↓
   Identify optional columns (present in SOME files)
   ↓
   Detect type conflicts
   ↓
   Return schema validation report

2. BATCH PROCESSING
   POST /api/batch/{batch_id}/process
   ↓
   Celery chain:
     process_batch_files.s() → consolidate_files_task.s()
   ↓

   STEP A: process_batch_files
     For each file (sequential):
       - Stream read in chunks (1000 rows)
       - Apply refinery cleaning
       - Save chunks to temporary parquet
       - Merge chunks for file
       - Update progress in Redis

   STEP B: consolidate_files_task
     - Read all file parquets (PyArrow)
     - Concatenate efficiently
     - Add source_file column (track origin)
     - Cross-file deduplication (content_only on clean*)
     - Generate LLM input JSON
     - Save consolidated parquet
     - Update batch state in Redis + PostgreSQL

3. LLM CLASSIFICATION (same as single-file step 4)

4. VALIDATION & REFINEMENT (same as single-file steps 5-9)
```

---

## Go Migration Strategy

### 1. High-Level Architecture for Go

```
panel-datainspector-go/
├── cmd/
│   ├── api/              # HTTP server
│   ├── worker/           # Task worker
│   └── migrate/          # DB migrations
├── internal/
│   ├── api/
│   │   ├── handlers/     # HTTP handlers (12 routers)
│   │   ├── middleware/   # Logging, CORS
│   │   └── websocket/    # WebSocket handlers
│   ├── services/
│   │   ├── llm/          # LLM classifier (CRITICAL)
│   │   ├── batch/        # Batch processing
│   │   ├── cleaning/     # Refinery + deduplication
│   │   ├── validation/   # Sampling + iteration tracking
│   │   ├── metrics/      # W&B integration
│   │   └── storage/      # Redis + PostgreSQL
│   ├── models/           # Data models
│   ├── tasks/            # Background tasks
│   └── utils/            # Logger, serialization
├── pkg/
│   ├── dataframe/        # Pandas-like DataFrame (gota or custom)
│   ├── refinery/         # Text cleaning pipelines
│   └── openai/           # OpenAI client wrapper
├── migrations/           # SQL migrations
└── docker-compose.yml
```

### 2. Critical Components to Prioritize

**Phase 1: Core Data Processing** (Weeks 1-2)
1. DataFrame abstraction (consider `gota` or custom)
2. Refinery pipeline (v1/v2/v3)
3. Universal deduplication service
4. File I/O (Excel/CSV/JSON streaming)

**Phase 2: LLM Integration** (Weeks 3-4)
1. OpenAI async client (use `go-openai`)
2. LLM classifier with chunking
3. Parallel processing with goroutines + semaphore
4. Response validation + normalization
5. Count mismatch handling (CRITICAL!)

**Phase 3: API Layer** (Weeks 5-6)
1. HTTP handlers (Gin or Chi)
2. Middleware (logging, CORS)
3. WebSocket support
4. Session management (Redis)

**Phase 4: Async Processing** (Weeks 7-8)
1. Task queue (Asynq or Machinery)
2. Background workers
3. Task monitoring
4. Progress tracking

**Phase 5: Batch Processing** (Weeks 9-10)
1. Multi-file orchestration
2. Schema validation with caching
3. Streaming consolidation
4. Cross-file deduplication

**Phase 6: Validation & Metrics** (Weeks 11-12)
1. Sampling algorithms
2. Iteration tracking
3. Comparative metrics
4. W&B integration

### 3. Technology Recommendations for Go

**Web Framework**:
- **Gin** (fast, popular) or **Chi** (lightweight, stdlib-like)

**DataFrame**:
- **gota** (pandas-like) or **custom struct with methods**
```go
type DataFrame struct {
    Columns []string
    Rows    []map[string]interface{}
}

func (df *DataFrame) Filter(col string, values []string) *DataFrame
func (df *DataFrame) Sample(n int) *DataFrame
func (df *DataFrame) DropDuplicates(cols []string) *DataFrame
```

**OpenAI Client**:
- **sashabaranov/go-openai**
```go
import "github.com/sashabaranov/go-openai"

client := openai.NewClient(apiKey)
resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
    Model: "gpt-4o-mini",
    Messages: []openai.ChatCompletionMessage{
        {Role: "user", Content: prompt},
    },
})
```

**Task Queue**:
- **Asynq** (Redis-based, similar to Celery)
```go
import "github.com/hibiken/asynq"

// Task definition
type ProcessFileTask struct {
    BatchID string
    FilePath string
}

// Worker
func ProcessFile(ctx context.Context, t *asynq.Task) error {
    var payload ProcessFileTask
    json.Unmarshal(t.Payload(), &payload)
    // Process file...
    return nil
}

// Enqueue
client := asynq.NewClient(asynq.RedisClientOpt{Addr: "localhost:6379"})
task := asynq.NewTask("process_file", payloadBytes)
client.Enqueue(task)
```

**Redis**:
- **go-redis/redis**
```go
import "github.com/go-redis/redis/v8"

rdb := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
    DB: 1,
})

// Set batch state
rdb.Set(ctx, "batch_state:"+batchID, jsonData, 24*time.Hour)

// Get batch state
val, err := rdb.Get(ctx, "batch_state:"+batchID).Result()
```

**PostgreSQL**:
- **sqlx** (SQL extensions) + **lib/pq** (driver)
```go
import (
    "github.com/jmoiron/sqlx"
    _ "github.com/lib/pq"
)

db, err := sqlx.Connect("postgres", "user=admin dbname=datainspector sslmode=disable")

type BatchPipeline struct {
    ID        string    `db:"id"`
    BatchID   string    `db:"batch_id"`
    Status    string    `db:"status"`
    CreatedAt time.Time `db:"created_at"`
}

var batches []BatchPipeline
db.Select(&batches, "SELECT * FROM batch_pipelines WHERE status = $1", "processing")
```

**Excel/CSV**:
- **tealeg/xlsx** (Excel)
- **encoding/csv** (CSV - stdlib)
- **tidwall/gjson** (JSON parsing)

**WebSocket**:
- **gorilla/websocket**
```go
import "github.com/gorilla/websocket"

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func BatchWebSocket(w http.ResponseWriter, r *http.Request) {
    conn, _ := upgrader.Upgrade(w, r, nil)
    defer conn.Close()

    ticker := time.NewTicker(500 * time.Millisecond)
    for range ticker.C {
        state := getBatchState(batchID)
        conn.WriteJSON(state)

        if state.Status == "completed" {
            break
        }
    }
}
```

### 4. Critical Go Implementation Patterns

**LLM Classifier with Goroutines**:

```go
func ClassifyBatch(ctx context.Context, df *DataFrame, config LLMConfig) (*DataFrame, *Metadata, error) {
    // 1. Split into chunks
    chunks := splitIntoChunks(df, config.ChunkSize)

    // 2. Create semaphore for concurrency control
    sem := make(chan struct{}, config.ConcurrencyLimit)

    // 3. Process chunks in parallel
    results := make(chan ChunkResult, len(chunks))
    errChan := make(chan error, len(chunks))

    for i, chunk := range chunks {
        go func(idx int, c *DataFrame) {
            sem <- struct{}{}        // Acquire semaphore
            defer func() { <-sem }() // Release semaphore

            result, err := processChunk(ctx, c, idx, config)
            if err != nil {
                errChan <- err
                return
            }
            results <- result
        }(i, chunk)
    }

    // 4. Collect results
    allResults := []map[string]interface{}{}
    for i := 0; i < len(chunks); i++ {
        select {
        case result := <-results:
            if !result.Success {
                return nil, nil, fmt.Errorf("chunk %d failed: %s", result.ChunkIndex, result.Error)
            }
            allResults = append(allResults, result.Results...)
        case err := <-errChan:
            return nil, nil, err
        }
    }

    // 5. Merge results using row index
    resultDF := mergeByRowIndex(df, allResults)

    // 6. Capture baseline metrics
    metadata := captureBaselineMetrics(df, resultDF)

    return resultDF, metadata, nil
}

func processChunk(ctx context.Context, chunk *DataFrame, chunkIndex int, config LLMConfig) (ChunkResult, error) {
    retryCount := 0

    for retryCount < config.MaxRetries {
        // 1. Build payload
        payload := map[string]interface{}{
            "entries": chunk.ToRecords(),
        }

        // 2. Format prompt
        prompt := formatPrompt(payload, config.CustomPrompt, config.CleanFields)

        // 3. Call OpenAI API
        client := openai.NewClient(config.APIKey)
        resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
            Model: config.Model,
            Messages: []openai.ChatCompletionMessage{
                {Role: "user", Content: prompt},
            },
        })

        if err != nil {
            retryCount++
            if retryCount >= config.MaxRetries {
                return ChunkResult{Success: false, Error: err.Error()}, err
            }
            time.Sleep(time.Duration(500*math.Pow(2, float64(retryCount))) * time.Millisecond)
            continue
        }

        // 4. Parse JSON response
        content := resp.Choices[0].Message.Content
        var parsed map[string]interface{}
        if err := json.Unmarshal([]byte(content), &parsed); err != nil {
            retryCount++
            continue
        }

        results := parsed["results"].([]interface{})

        // 5. CRITICAL: Validate count
        if len(results) != len(chunk.Rows) {
            results = fixCountMismatch(results, chunk, config.CleanFields[0])
        }

        // 6. Normalize field names
        normalized := make([]map[string]interface{}, len(results))
        for i, r := range results {
            normalized[i] = normalizeLLMResponse(r.(map[string]interface{}), chunkIndex)
            normalized[i]["_row_index"] = chunk.Rows[i]["_row_index"]
        }

        return ChunkResult{Success: true, Results: normalized}, nil
    }

    return ChunkResult{Success: false, Error: "max retries exceeded"}, errors.New("max retries exceeded")
}
```

**Streaming File Processing**:

```go
func ProcessFileStream(filePath string, chunkSize int, processFunc func(*DataFrame) error) error {
    ext := filepath.Ext(filePath)

    if ext == ".xlsx" {
        // Excel streaming (read sheet row by row)
        xlsxFile, err := xlsx.OpenFile(filePath)
        if err != nil {
            return err
        }

        sheet := xlsxFile.Sheets[0]
        headers := []string{}
        for _, cell := range sheet.Rows[0].Cells {
            headers = append(headers, cell.String())
        }

        rows := []map[string]interface{}{}
        for i, row := range sheet.Rows[1:] {
            rowData := make(map[string]interface{})
            for j, cell := range row.Cells {
                rowData[headers[j]] = cell.String()
            }
            rows = append(rows, rowData)

            // Process chunk
            if len(rows) >= chunkSize {
                df := &DataFrame{Columns: headers, Rows: rows}
                if err := processFunc(df); err != nil {
                    return err
                }
                rows = []map[string]interface{}{} // Reset
            }
        }

        // Process remaining rows
        if len(rows) > 0 {
            df := &DataFrame{Columns: headers, Rows: rows}
            return processFunc(df)
        }

    } else if ext == ".csv" {
        // CSV streaming (use csv.Reader)
        file, err := os.Open(filePath)
        if err != nil {
            return err
        }
        defer file.Close()

        reader := csv.NewReader(file)
        headers, _ := reader.Read() // First row

        rows := []map[string]interface{}{}
        for {
            record, err := reader.Read()
            if err == io.EOF {
                break
            }

            rowData := make(map[string]interface{})
            for i, val := range record {
                rowData[headers[i]] = val
            }
            rows = append(rows, rowData)

            // Process chunk
            if len(rows) >= chunkSize {
                df := &DataFrame{Columns: headers, Rows: rows}
                if err := processFunc(df); err != nil {
                    return err
                }
                rows = []map[string]interface{}{} // Reset
            }
        }

        // Process remaining
        if len(rows) > 0 {
            df := &DataFrame{Columns: headers, Rows: rows}
            return processFunc(df)
        }
    }

    return nil
}
```

**Batch State Management**:

```go
type BatchStateManager struct {
    redis *redis.Client
    db    *sqlx.DB
}

func (bsm *BatchStateManager) SetBatchState(ctx context.Context, batchID string, state interface{}) error {
    // Serialize state
    stateJSON, err := json.Marshal(state)
    if err != nil {
        return err
    }

    // Store in Redis (24h TTL)
    key := "batch_state:" + batchID
    if err := bsm.redis.Set(ctx, key, stateJSON, 24*time.Hour).Err(); err != nil {
        return err
    }

    // Async persist to PostgreSQL
    go bsm.persistToDB(batchID, state)

    return nil
}

func (bsm *BatchStateManager) GetBatchState(ctx context.Context, batchID string) (map[string]interface{}, error) {
    // Try Redis first
    key := "batch_state:" + batchID
    val, err := bsm.redis.Get(ctx, key).Result()
    if err == nil {
        var state map[string]interface{}
        json.Unmarshal([]byte(val), &state)
        return state, nil
    }

    // Try PostgreSQL recovery
    if err == redis.Nil {
        return bsm.recoverFromDB(ctx, batchID)
    }

    return nil, err
}

func (bsm *BatchStateManager) persistToDB(batchID string, state interface{}) {
    ctx := context.Background()

    stateJSON, _ := json.Marshal(state)

    _, err := bsm.db.ExecContext(ctx, `
        INSERT INTO batch_pipelines (batch_id, state_snapshot, last_updated)
        VALUES ($1, $2, NOW())
        ON CONFLICT (batch_id) DO UPDATE SET
            state_snapshot = $2,
            last_updated = NOW()
    `, batchID, stateJSON)

    if err != nil {
        log.Printf("Failed to persist batch state to DB: %v", err)
    }
}
```

### 5. Testing Strategy

**Unit Tests** (70% coverage):
```go
func TestLLMClassifier_CountMismatch(t *testing.T) {
    // Given: Chunk with 10 items
    chunk := &DataFrame{
        Columns: []string{"cleanLineDescription"},
        Rows: []map[string]interface{}{
            {"cleanLineDescription": "text1", "_row_index": 0},
            {"cleanLineDescription": "text2", "_row_index": 1},
            // ... 10 total
        },
    }

    // When: LLM returns only 5 results
    results := []interface{}{
        map[string]interface{}{"cleanLineDescription": "text1", "category": "Pop"},
        // ... 5 total
    }

    // Then: fixCountMismatch should fill missing with defaults
    fixed := fixCountMismatch(results, chunk, "cleanLineDescription")

    assert.Equal(t, 10, len(fixed))
    assert.Equal(t, "Pop", fixed[0]["category"])
    assert.Equal(t, "Indeterminado", fixed[5]["category"]) // Default
}

func TestRefinery_SpanishCleaning(t *testing.T) {
    refinery := NewRefinery("v3")

    input := "INSTALACIÓN VINIL   COCA-COLA"
    expected := "instalacion vinil coca cola"

    result := refinery.Process(input)
    assert.Equal(t, expected, result)
}

func TestDeduplication_ContentOnly(t *testing.T) {
    df := &DataFrame{
        Columns: []string{"ID", "cleanLineDescription"},
        Rows: []map[string]interface{}{
            {"ID": "1", "cleanLineDescription": "text1"},
            {"ID": "2", "cleanLineDescription": "text1"}, // Duplicate content
            {"ID": "3", "cleanLineDescription": "text2"},
        },
    }

    dedupSvc := NewDeduplicationService()
    result := dedupSvc.DeduplicateByColumns(df, []string{"cleanLineDescription"}, "content_only")

    assert.Equal(t, 2, len(result.DeduplicatedDF.Rows)) // 1 duplicate removed
}
```

**Integration Tests**:
```go
func TestBatchProcessing_EndToEnd(t *testing.T) {
    // Setup: Create test files
    testDir := createTestDirectory(t)
    defer os.RemoveAll(testDir)

    // Step 1: Inspect directory
    batchID, _ := InspectDirectory(testDir, []string{"*.xlsx"})

    // Step 2: Process batch
    ProcessBatch(batchID, BatchProcessRequest{
        SelectedColumns: []string{"LineDescription"},
        RefineryType: "v3",
        DeduplicationStrategy: "content_only",
    })

    // Step 3: Check results
    results := GetBatchResults(batchID)
    assert.NotNil(t, results.ConsolidatedFilePath)
    assert.Greater(t, results.TotalRecordsProcessed, 0)
}
```

---

## Conclusion

This document provides a comprehensive blueprint for refactoring Panel-Datainspector from Python to Go. The most critical components to get right are:

1. **LLM Classifier** - Parallel chunk processing with exact count validation
2. **Deduplication** - Universal service with content-only strategy
3. **Batch Processing** - Streaming file processing with cross-file consolidation
4. **State Management** - Redis + PostgreSQL hybrid for resilience
5. **Iteration Tracking** - Accumulating validated examples across iterations

**Key Invariants to Maintain**:
- LLM output count MUST equal input count (1:1 mapping)
- Use row indices for merging (NOT content-based joins)
- Separate accuracy metrics from ambiguous records (score != -1)
- Preserve ALL clean fields dynamically (not hardcoded)
- Handle response count mismatches gracefully (fill with defaults)

**Performance Targets**:
- Process 10K records in < 2 minutes
- Support files > 500MB with streaming
- Parallel LLM processing with 3-5 concurrent chunks
- Memory usage < 4GB for typical workloads

Good luck with the Go refactoring! 🚀