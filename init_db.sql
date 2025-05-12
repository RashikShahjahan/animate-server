-- Create database
-- Note: This must be run as a PostgreSQL superuser
-- Run this separately if needed: CREATE DATABASE animations;

-- The rest of this script should be run against the animations database
-- psql -U postgres -d animations -f init_db.sql

-- Create table for animations
CREATE TABLE IF NOT EXISTS animations (
    id VARCHAR(32) PRIMARY KEY,
    code TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create index on id for faster lookups
CREATE INDEX IF NOT EXISTS idx_animations_id ON animations(id);

-- Add comment on table
COMMENT ON TABLE animations IS 'Stores p5.js animation codes with unique IDs';

-- Add description column if it doesn't exist
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'animations'
        AND column_name = 'description'
    ) THEN
        ALTER TABLE animations ADD COLUMN description TEXT;
    END IF;
END $$;

-- Add username column to users table if it doesn't exist
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'users'
        AND column_name = 'username'
    ) THEN
        ALTER TABLE users ADD COLUMN username VARCHAR(255);
    END IF;
END $$; 