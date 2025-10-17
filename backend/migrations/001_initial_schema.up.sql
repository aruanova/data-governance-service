-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Batches table: stores each file processing session
CREATE TABLE batches (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    original_filename VARCHAR(500) NOT NULL,
    file_path TEXT,
    file_hash VARCHAR(64) UNIQUE NOT NULL, -- SHA256 hash for idempotency
    status VARCHAR(50) NOT NULL DEFAULT 'uploaded',
    total_records INTEGER DEFAULT 0,
    processed_records INTEGER DEFAULT 0,
    config JSONB,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,

    -- Status: uploaded, cleaning, llm_processing, validating, completed, failed
    CONSTRAINT valid_status CHECK (status IN ('uploaded', 'cleaning', 'llm_processing', 'validating', 'completed', 'failed'))
);

-- Indexes for querying
CREATE INDEX idx_batches_status ON batches(status);
CREATE INDEX idx_batches_created_at ON batches(created_at DESC);
CREATE UNIQUE INDEX idx_batches_file_hash ON batches(file_hash);

-- Prompts table: customizable LLM prompts
CREATE TABLE prompts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    label VARCHAR(255) UNIQUE NOT NULL,
    template TEXT NOT NULL,
    categories JSONB NOT NULL,
    is_default BOOLEAN DEFAULT FALSE,
    created_by VARCHAR(255),
    version INTEGER DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Index for quick default lookup
CREATE INDEX idx_prompts_default ON prompts(is_default) WHERE is_default = TRUE;
CREATE INDEX idx_prompts_label ON prompts(label);

-- Classifications table: stores LLM classification results
CREATE TABLE classifications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    batch_id UUID NOT NULL REFERENCES batches(id) ON DELETE CASCADE,
    row_index INTEGER NOT NULL,
    original_data JSONB NOT NULL,
    cleaned_data JSONB NOT NULL,
    category VARCHAR(255),
    reason TEXT,
    confidence_score DECIMAL(5,4),
    llm_provider VARCHAR(50),
    llm_model VARCHAR(100),
    tokens_used INTEGER,
    processing_time_ms INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Idempotency: prevent duplicate classifications for same row
    CONSTRAINT unique_batch_row UNIQUE(batch_id, row_index),
    CONSTRAINT fk_batch FOREIGN KEY (batch_id) REFERENCES batches(id) ON DELETE CASCADE
);

CREATE INDEX idx_classifications_batch ON classifications(batch_id);
CREATE INDEX idx_classifications_category ON classifications(category);
CREATE INDEX idx_classifications_confidence ON classifications(confidence_score DESC);

-- Validations table: manual validation samples
CREATE TABLE validations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    batch_id UUID NOT NULL REFERENCES batches(id) ON DELETE CASCADE,
    classification_id UUID NOT NULL REFERENCES classifications(id) ON DELETE CASCADE,
    sampling_strategy VARCHAR(100),
    user_feedback VARCHAR(50),
    corrected_category VARCHAR(255),
    user_notes TEXT,
    idempotency_key VARCHAR(64) UNIQUE, -- For API idempotency
    validated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT fk_batch_validation FOREIGN KEY (batch_id) REFERENCES batches(id) ON DELETE CASCADE,
    CONSTRAINT fk_classification FOREIGN KEY (classification_id) REFERENCES classifications(id) ON DELETE CASCADE,
    -- Feedback: correct, incorrect, uncertain
    CONSTRAINT valid_feedback CHECK (user_feedback IN ('correct', 'incorrect', 'uncertain')),
    -- Prevent duplicate validations for same classification
    CONSTRAINT unique_classification_validation UNIQUE(classification_id)
);

CREATE INDEX idx_validations_batch ON validations(batch_id);
CREATE INDEX idx_validations_feedback ON validations(user_feedback);
CREATE INDEX idx_validations_idempotency ON validations(idempotency_key) WHERE idempotency_key IS NOT NULL;

-- Iterations table: track prompt refinement iterations
CREATE TABLE iterations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    batch_id UUID NOT NULL REFERENCES batches(id) ON DELETE CASCADE,
    iteration_number INTEGER NOT NULL,
    prompt_id UUID REFERENCES prompts(id),
    prompt_changes TEXT,
    metrics JSONB,
    accuracy_delta DECIMAL(5,2),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT fk_batch_iteration FOREIGN KEY (batch_id) REFERENCES batches(id) ON DELETE CASCADE,
    CONSTRAINT unique_batch_iteration UNIQUE(batch_id, iteration_number)
);

CREATE INDEX idx_iterations_batch ON iterations(batch_id, iteration_number);

-- Sessions table: user workflow state
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    batch_id UUID REFERENCES batches(id) ON DELETE CASCADE,
    user_id VARCHAR(255),
    current_step VARCHAR(50) NOT NULL DEFAULT 'upload',
    state JSONB,
    last_activity TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE,

    CONSTRAINT fk_batch_session FOREIGN KEY (batch_id) REFERENCES batches(id) ON DELETE CASCADE
);

CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_batch ON sessions(batch_id);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

-- Deduplication tracking table
CREATE TABLE dedup_hashes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    batch_id UUID NOT NULL REFERENCES batches(id) ON DELETE CASCADE,
    hash VARCHAR(64) NOT NULL,
    original_row_index INTEGER NOT NULL,
    kept BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT fk_batch_dedup FOREIGN KEY (batch_id) REFERENCES batches(id) ON DELETE CASCADE
);

CREATE INDEX idx_dedup_batch_hash ON dedup_hashes(batch_id, hash);
CREATE INDEX idx_dedup_kept ON dedup_hashes(kept);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Triggers for updated_at
CREATE TRIGGER update_batches_updated_at BEFORE UPDATE ON batches
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_prompts_updated_at BEFORE UPDATE ON prompts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_classifications_updated_at BEFORE UPDATE ON classifications
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Insert default prompt
INSERT INTO prompts (name, label, template, categories, is_default) VALUES (
    'Default Spanish Classification',
    'default-spanish-v1',
    'Clasifica las siguientes líneas de detalle de auxiliares empresariales en las categorías apropiadas. Responde en formato JSON con exactamente el mismo número de resultados que entradas recibidas.',
    '[
        {"id": 1, "name": "Publicidad", "description": "Gastos en publicidad, marketing, promociones", "keywords": ["promo", "publicidad", "marketing", "anuncio"]},
        {"id": 2, "name": "Material POP", "description": "Material punto de venta", "keywords": ["pop", "display", "material"]},
        {"id": 3, "name": "Impresiones", "description": "Servicios de impresión", "keywords": ["impresion", "imprenta"]},
        {"id": 4, "name": "Medios", "description": "Gastos en medios de comunicación", "keywords": ["tv", "radio", "medios"]},
        {"id": 5, "name": "Indeterminado", "description": "No se puede clasificar con certeza", "keywords": []}
    ]'::jsonb,
    TRUE
);