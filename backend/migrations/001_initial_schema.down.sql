-- Drop triggers
DROP TRIGGER IF EXISTS update_classifications_updated_at ON classifications;
DROP TRIGGER IF EXISTS update_prompts_updated_at ON prompts;
DROP TRIGGER IF EXISTS update_batches_updated_at ON batches;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables in reverse order (respecting foreign keys)
DROP TABLE IF EXISTS dedup_hashes;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS iterations;
DROP TABLE IF EXISTS validations;
DROP TABLE IF EXISTS classifications;
DROP TABLE IF EXISTS prompts;
DROP TABLE IF EXISTS batches;

-- Drop extension
DROP EXTENSION IF EXISTS "uuid-ossp";