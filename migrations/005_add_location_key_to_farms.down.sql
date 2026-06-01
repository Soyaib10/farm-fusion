-- Remove location_key column from farms table
DROP INDEX IF EXISTS idx_farms_location_key;
ALTER TABLE farms DROP COLUMN IF EXISTS location_key;
