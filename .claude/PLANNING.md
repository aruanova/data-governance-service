# Panel-Datainspector - Plan de Refactorización (Monorepo)

## 📋 Estado Actual
- **Stack Backend**: FastAPI + Celery + Redis + PostgreSQL + OpenAI
- **Stack Frontend**: Next.js 15.3.5 + React 19 + TypeScript
- **Propósito**: Clasificación de líneas CFDI usando LLM con refinamiento iterativo
- **Complejidad**: 12 routers, 25+ servicios, procesamiento async distribuido

## 🎯 Objetivos de la Refactorización
1. Migrar backend de Python/FastAPI a Go
2. Cambiar a Svelte con Skeleton (solo ajustar endpoints si es necesario)
3. Estructura monorepo para facilitar desarrollo
4. Mejorar performance y reducir uso de memoria
5. Simplificar arquitectura manteniendo funcionalidad completa

## 🏗️ Estructura Monorepo Propuesta

```
data-governance-service/
├── .claude/                    # Documentación y contexto de Claude
│   ├── PLANNING.md
│   └── CLAUDE.md
├── backend/                    # Backend en Go
│   ├── cmd/
│   │   ├── api/               # API REST server
│   │   └── worker/            # Worker para procesamiento async
│   ├── internal/
│   │   ├── api/               # HTTP handlers
│   │   │   ├── handlers/      # Controladores REST
│   │   │   ├── middleware/    # Auth, CORS, logging
│   │   │   └── websocket/     # WebSocket handlers
│   │   ├── core/              # Dominio central
│   │   │   ├── models/        # Modelos de dominio
│   │   │   ├── ports/         # Interfaces (hexagonal)
│   │   │   └── services/      # Lógica de negocio
│   │   ├── infrastructure/    # Implementaciones externas
│   │   │   ├── database/      # PostgreSQL
│   │   │   ├── cache/         # Redis
│   │   │   ├── storage/       # File system / S3
│   │   │   ├── llm/           # OpenAI client
│   │   │   └── queue/         # Task queue (Asynq)
│   │   └── pkg/               # Utilidades compartidas
│   ├── migrations/            # SQL migrations
│   ├── configs/               # Configuraciones
│   └── tests/                 # Tests de integración
├── frontend/                  # Frontend (A definir si Svelte)
│   ├── app/
│   ├── components/
│   ├── lib/
│   └── public/
├── shared/                    # Compartido entre front y back
│   ├── types/                # TypeScript types que matchean Go structs
│   └── api-spec/             # OpenAPI/Swagger specs
├── scripts/                   # Scripts de build y deployment
├── docker/                    # Dockerfiles y compose
├── .github/                   # CI/CD workflows
└── docs/                      # Documentación del proyecto
```

## 🔄 Servicios Go Propuestos

### 1. API Service (backend/cmd/api)
- **Responsabilidad**: Exponer REST API, manejar uploads, WebSockets
- **Puertos**: 8080 (HTTP), 8081 (WebSocket)
- **Similar a routers Python**: pipeline.py, cleaning.py, validation.py, batch.py

### 2. Worker Service (backend/cmd/worker)
- **Responsabilidad**: Procesar tareas async, LLM calls, batch processing
- **Similar a**: Celery workers actuales
- **Queue**: Asynq (Redis-based) o Temporal
- **Resiliencia**: Checkpointing, health checks, graceful shutdown

### 3. Servicios de Dominio (backend/internal/core/services)
```
services/
├── upload/              # Gestión de uploads y archivos ZIP
├── schema_inspector/    # Inspección y análisis de archivos
├── refinery/           # Sistema modular de limpieza de texto
│   ├── v1_standard/    # Versión para datos mexicanos
│   ├── v2_aggressive/  # Limpieza más agresiva
│   └── registry/       # Registro de versiones
├── cleaning/           # Limpieza y deduplicación
│   ├── deduplication/  # Sistema two-level de dedup
│   └── json_generator/ # Generador de JSON para LLM
├── prompts/            # Gestión de prompts customizables
│   ├── storage/        # Almacenamiento de prompts
│   └── versioning/     # Versionado de prompts
├── llm/               # Clasificación con LLM
├── validation/        # Muestreo y validación
├── batch/             # Procesamiento multi-archivo
├── iteration/         # Tracking de iteraciones
├── metrics/           # Captura de métricas
├── checkpoint/        # Sistema de checkpointing
├── recovery/          # Recuperación de trabajos fallidos
└── session/           # Gestión de sesiones
```

## 📊 Flujo de Procesamiento de Archivos

### Pipeline Pre-LLM
```mermaid
graph LR
    A[Upload ZIP] --> B[Schema Inspection]
    B --> C[Refinery Cleaning]
    C --> D[Deduplication]
    D --> E[JSON Generation]
    E --> F[LLM Processing]
```

### Etapas Detalladas

#### 1. Upload & Extracción
- Recepción de archivos ZIP con múltiples formatos
- Soporte para: CSV, XLSX, JSON, JSONL, NDJSON, JSONNL
- Streaming para archivos grandes
- Validación de esquemas

#### 2. Schema Inspection
- Detección automática de columnas
- Inferencia de tipos de datos
- Análisis de calidad de datos
- Sampling para preview

#### 3. Refinery (Sistema Modular)
- **Arquitectura Plugin**: Versiones intercambiables
- **v1-standard**: Limpieza para datos mexicanos
- **v2-aggressive**: Limpieza más estricta
- **Nodos de Procesamiento**:
  - Remover códigos con prefijos (ART###)
  - Remover meses en español
  - Normalización de caracteres especiales
  - Remover palabras cortas
  - Aplicar whitelist/blacklist

#### 4. Deduplicación Two-Level
- **Nivel 1**: Deduplicación dentro del batch
- **Nivel 2**: Deduplicación universal (cross-session)
- Estrategias: exact, fuzzy, universal
- Usa columnas "clean" generadas por Refinery

#### 5. JSON Generation
- Creación de estructura optimizada para LLM
- Inclusión de _row_index para tracking
- Solo campos "clean" para reducir tokens
- Metadata para contexto

## 🤖 Sistema Multi-Proveedor LLM

### Proveedores Soportados
- **OpenAI**: GPT-4, GPT-4o-mini, GPT-3.5-turbo
- **Google Gemini**: Gemini-Pro, Gemini-1.5-Pro
- **Extensible**: Factory pattern para agregar nuevos proveedores

### Características
- **Selección Dinámica**: Usuario escoge proveedor por request
- **Fallback Automático**: Si un proveedor falla, notificar al usuario y darle a escoger usar una alternativa
- **Configuración por Proveedor**:
  - API Keys independientes
  - Modelos específicos
  - Rate limits y retry policies
- **Métricas Comparativas**:
  - Tiempo de respuesta
  - Costo por tokens
  - Calidad de clasificación

## 🎨 Sistema de Prompts Customizables

### Características Clave
- **100% Customizable**: NO hay prompt default
- **Gestión de Prompts**:
  - Crear, editar, eliminar prompts
  - Asignar labels descriptivos
  - Versionado automático
  - Compartir entre usuarios
- **Categorías Dinámicas**:
  - Usuario define sus propias categorías
  - Puede importar/exportar sets de categorías
  - Prioridades configurables
- **Storage**:
  - PostgreSQL para persistencia
  - Redis para cache
  - Historial de versiones

### Estructura de Prompt
```json
{
  "id": "uuid",
  "name": "Mi Prompt OXXO v3",
  "label": "Clasificación gastos OXXO 2024",
  "template": "Texto del prompt customizado...",
  "categories": [
    {
      "id": 1,
      "name": "Categoría Custom 1",
      "description": "Descripción",
      "keywords": ["palabra1", "palabra2"]
    }
  ],
  "created_by": "user_id",
  "version": 3,
  "is_active": true
}
```

## 📊 Fases de Migración

### Fase 0: Setup Monorepo ✅ COMPLETADA
- [x] Crear estructura .claude
- [x] Definir estructura de carpetas
- [x] Setup Go modules
- [x] Configurar herramientas (air para hot reload)
- [ ] Docker Compose para desarrollo (pendiente)

### Fase 1: Core Infrastructure ✅ COMPLETADA
- [x] Conexión PostgreSQL con GORM (cambio de pgx a GORM)
- [x] Conexión Redis (go-redis/v9)
- [x] Logging estructurado (slog)
- [x] Config management (viper)
- [x] Migrations setup (SQL migrations manuales)
- [x] Task Queue (Asynq)
- [x] Domain models (7 modelos con GORM)
- [x] Testing setup (testcontainers-go con PostgreSQL)

### Fase 2: Upload & Cleaning Pipeline ✅ COMPLETADA (100%)
- [x] Storage local para uploads (FileMetadata, SHA256 hashing, cleanup - 9 tests)
- [x] Parsers de archivos multi-formato (CSV, Excel, JSON, JSONL, NDJSON, JSONNL - 27 tests)
- [x] Parsing streaming de archivos grandes (buffers configurables, context-aware)
- [x] Pipeline de limpieza (Refinery v1 - 42 tests pasando)
- [x] Deduplicación universal (two-level: batch + cross-session - 10 tests)
- [x] Generación de JSON para LLM (21 tests - token-optimized, chunking, validation)

### Fase 3: LLM Integration (Semana 5-6)
- [ ] Cliente OpenAI
- [ ] Cliente Gemini
- [ ] Chunking dinámico configurable (LLM_DISTRIBUTED_CHUNK_SIZE)
- [ ] Worker pool configurable para procesamiento paralelo
- [ ] Manejo de respuestas y normalización
- [ ] Count mismatch handling
- [ ] Retry logic con backoff

### Fase 4: Task Queue System ⚠️ PARCIAL
- [x] Setup Asynq (ya implementado en Fase 1)
- [ ] Migrar lógica de Celery tasks
- [ ] Progress tracking
- [ ] WebSocket updates

### Fase 5: Validation & Refinement (Semana 9-10)
- [ ] Sampling strategies
- [ ] Validation submission
- [ ] Prompt refinement
- [ ] Iteration tracking
- [ ] Comparative metrics

### Fase 6: Batch Processing (Semana 11-12)
- [ ] Directory scanning
- [ ] Schema validation
- [ ] Multi-file processing
- [ ] Consolidation
- [ ] Streaming para archivos grandes

## 🛠️ Stack Tecnológico Definitivo

### Backend (Go)
- **Web Framework**: Gin
- **Task Queue**: Asynq (simple, Redis)
- **Database**: GORM (ORM completo)
- **Cache**: go-redis/v9
- **LLM Providers**:
  - OpenAI: sashabaranov/go-openai
  - Gemini: google/generative-ai-go
  - Factory Pattern para multi-proveedor
- **Excel**: excelize/v2
- **Validation**: go-playground/validator/v10
- **Config**: spf13/viper
- **Logging**: slog (stdlib)
- **Metrics**: Prometheus

### Infraestructura
- **PostgreSQL**: v15+ (igual que actual)
- **Redis**: v7+ (igual que actual)
- **Docker**: Multi-stage builds
- **CI/CD**: GitHub Actions

## ❓ Decisiones Técnicas Pendientes

### Alta Prioridad
1. **Web Framework**:
   - Gin (popular, buen balance)

2. **Task Queue**:
   - Asynq (simple, suficiente para el caso)

3. **SQL Approach**:
   - GORM (ORM completo)

### Media Prioridad ✅ RESUELTAS
4. **Estructura del código**:
   - ✅ Hexagonal/Clean Architecture (implementada)

5. **Testing Strategy**:
   - ✅ testify para assertions (implementado)
   - ⏳ mockery para mocks (pendiente)
   - ✅ testcontainers para integration (implementado)

## 🎯 Optimizaciones Clave vs Python

### Performance
```go
// Streaming de archivos grandes sin cargar en memoria
func ProcessLargeFile(reader io.Reader) error {
    scanner := bufio.NewScanner(reader)
    batch := make([]Record, 0, 1000)

    for scanner.Scan() {
        record := ParseRecord(scanner.Text())
        batch = append(batch, record)

        if len(batch) >= 1000 {
            processBatch(batch)
            batch = batch[:0] // reuse slice
        }
    }
}
```

### Concurrencia Real
```go
// Procesamiento paralelo de chunks con workers pool
func ProcessChunks(chunks []Chunk) {
    workers := runtime.NumCPU()
    ch := make(chan Chunk, len(chunks))
    var wg sync.WaitGroup

    // Worker pool
    for i := 0; i < workers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for chunk := range ch {
                processWithLLM(chunk)
            }
        }()
    }

    // Feed work
    for _, chunk := range chunks {
        ch <- chunk
    }
    close(ch)
    wg.Wait()
}
```

## 📈 Métricas de Éxito
- [ ] 50% reducción en uso de memoria
- [ ] 2x mejora en throughput
- [ ] <100ms latencia P95 en API
- [ ] 80% cobertura de tests
- [ ] Zero downtime durante migración

## 🚀 Próximos Pasos Inmediatos
1. ✅ Crear estructura .claude
2. Definir decisiones técnicas clave
3. Crear CLAUDE.md con convenciones
4. Setup inicial del proyecto Go
5. Implementar primer endpoint como POC

## 📝 Notas Importantes
- Frontend se rediseña con Svelte/Skeleton
- API debe mantener compatibilidad con frontend actual
- Aprovechar monorepo para compartir tipos/specs
- Deployment será en AWS/GCP

---

## 🎯 CHECKPOINT #2 - File Processing & Parsers (17 Oct 2025)

### ✅ Completado en esta sesión

#### 1. **Local Storage System** (`internal/infrastructure/storage/`)
- **Archivo**: `local.go` (301 líneas)
- **Tests**: `local_test.go` (9 tests - 100% PASS)
- **Características**:
  - SaveUpload con SHA256 hashing para idempotencia
  - GetUpload para recuperar archivos
  - SaveProcessedFile para cleaned/llm_input/llm_response
  - DeleteUpload con cleanup de directorios
  - CleanupOldFiles basado en tiempo
  - ListProcessedFiles para auditoría
- **Estructura de directorios**:
  ```
  /tmp/uploads/
  ├── {uploadID}/original_file.xlsx
  /tmp/processed/
  ├── {uploadID}/
  │   ├── cleaned/cleaned_data.xlsx
  │   ├── llm_input/input.json
  │   └── llm_response/response.json
  ```

#### 2. **Multi-Format File Parsers** (`internal/infrastructure/parsers/`)
- **Archivos creados**:
  - `types.go` - Interfaces y tipos base
  - `csv_parser.go` - Parser CSV con variable fields
  - `excel_parser.go` - Parser Excel con excelize/v2
  - `json_parser.go` - Parser JSON (array y objetos)
  - `jsonl_parser.go` - Parser JSONL/NDJSON/JSONNL
  - `parser_factory.go` - Factory pattern con auto-detección
  - `parsers_test.go` - Suite completa de tests

- **Tests**: 27 tests - **100% PASS** ✅
  ```
  TestCSVParser_Parse ✓
  TestCSVParser_ParseStream ✓
  TestCSVParser_SkipEmptyRows ✓
  TestCSVParser_TrimWhitespace ✓
  TestCSVParser_MissingColumns ✓
  TestCSVParser_SupportedFormats ✓
  TestJSONParser_Parse ✓
  TestJSONParser_ParseStream ✓
  TestJSONParser_SupportedFormats ✓
  TestJSONLParser_Parse ✓
  TestJSONLParser_ParseStream ✓
  TestJSONLParser_SkipEmptyLines ✓
  TestJSONLParser_SkipMalformedLines ✓
  TestJSONLParser_SupportedFormats ✓
  TestJSONLParser_AllVariants (3 subtests) ✓
  TestParserFactory_GetParser (7 subtests) ✓
  TestParserFactory_GetParser_Unsupported ✓
  TestParserFactory_IsSupported ✓
  TestParserFactory_ParseFile (5 subtests) ✓
  TestParserFactory_SupportedFormats ✓
  TestParserConfig_MaxFileSize ✓
  TestContext_Cancellation ✓
  TestDefaultParserConfig ✓
  TestParseResult_Structure ✓
  ```

- **Formatos soportados**:
  - ✅ CSV (`.csv`) - encoding/csv con fields variables
  - ✅ Excel (`.xlsx`, `.xls`) - excelize/v2
  - ✅ JSON (`.json`) - encoding/json
  - ✅ JSONL (`.jsonl`) - Line-by-line streaming
  - ✅ NDJSON (`.ndjson`) - Newline Delimited JSON
  - ✅ JSONNL (`.jsonnl`) - JSON Newline variant

- **Características implementadas**:
  - **Streaming**: No carga archivos completos en memoria
  - **Context-aware**: Respeta context.Context para cancelación
  - **Configurable**: MaxFileSize, SkipEmptyRows, TrimWhitespace
  - **Resiliente**: Maneja columnas faltantes, líneas mal formadas
  - **Performance**: Buffers de 1MB para JSONL, reuso de slices

#### 3. **Dependencies agregadas**
```bash
go get github.com/xuri/excelize/v2  # Excel parsing
# Ya teníamos: testify, GORM, go-redis, asynq
```

### 📊 Progreso General

**Fase 1 (Core Infrastructure)**: ✅ 100% Completada
- PostgreSQL + GORM
- Redis cache
- Asynq queue
- Domain models (7)
- Testing con testcontainers

**Fase 2 (Upload & Cleaning Pipeline)**: 🔄 60% Completada
- ✅ Storage local (9 tests)
- ✅ File parsers (27 tests)
- ✅ Refinery v1 (42 tests)
- ⏳ Deduplicación (pendiente)
- ⏳ JSON generation para LLM (pendiente)

**Total de tests pasando**: 78 tests (9 storage + 27 parsers + 42 refinery)

### 🎯 Próximos pasos inmediatos

1. **Sistema de Deduplicación** (Fase 2)
   - Implementar two-level deduplication
   - Nivel 1: Within batch (exact matching)
   - Nivel 2: Universal cross-session (DedupHash table)
   - Estrategias: exact, fuzzy, universal

2. **JSON Generation para LLM** (Fase 2)
   - Estructura optimizada para tokens
   - Incluir _row_index para tracking
   - Solo campos "clean" para reducir tamaño
   - Metadata de contexto

3. **LLM Clients** (Fase 3)
   - OpenAI provider con sashabaranov/go-openai
   - Gemini provider con google/generative-ai-go
   - Factory pattern multi-proveedor
   - Retry logic y rate limiting

### 🏗️ Arquitectura actual

```
backend/
├── internal/
│   ├── core/
│   │   ├── domain/           ✅ 7 models (GORM)
│   │   └── refinery/         ✅ v1 implementado (42 tests)
│   └── infrastructure/
│       ├── database/         ✅ PostgreSQL + GORM
│       ├── cache/            ✅ Redis
│       ├── queue/            ✅ Asynq
│       ├── storage/          ✅ Local storage (9 tests)
│       └── parsers/          ✅ Multi-format (27 tests)
```

### 📝 Decisiones técnicas tomadas

1. **CSV Parser**: Usar `FieldsPerRecord = -1` para permitir columnas variables
2. **Excel Parser**: excelize/v2 por su madurez y soporte completo
3. **JSONL Variants**: Un solo parser maneja .jsonl, .ndjson, .jsonnl
4. **Streaming**: Buffers de 1MB para líneas JSONL, 10K records default para CSV
5. **Error handling**: Skip de líneas mal formadas, continuar procesamiento
6. **Storage paths**: Separación clara entre uploads/ y processed/

### 🔧 Herramientas y librerías en uso

- **GORM**: ORM completo para PostgreSQL
- **excelize/v2**: Parsing de Excel
- **go-redis/v9**: Cliente Redis
- **asynq**: Task queue
- **testify**: Assertions y require
- **testcontainers-go**: Integration tests con PostgreSQL real
- **slog**: Logging estructurado (stdlib)