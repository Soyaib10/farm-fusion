-- Add location_key column to farms table for weather data clustering
ALTER TABLE farms ADD COLUMN location_key VARCHAR(50);

-- Create index for efficient location-based queries
CREATE INDEX idx_farms_location_key ON farms(location_key);

-- Populate location_key for existing farms (round to 2 decimal places)
UPDATE farms 
SET location_key = CONCAT(
    ROUND(latitude::numeric, 2)::text, 
    '_', 
    ROUND(longitude::numeric, 2)::text
);

-- Make location_key NOT NULL after populating existing data
ALTER TABLE farms ALTER COLUMN location_key SET NOT NULL;
