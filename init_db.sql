-- Animate Server Database Initialization Script
-- This script sets up all necessary tables, indexes, and constraints

-- Create extensions if needed
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Set some session parameters
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;

-- Create table for animations if it doesn't exist
CREATE TABLE IF NOT EXISTS animations (
    id VARCHAR(32) PRIMARY KEY,
    code TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create table for users if it doesn't exist
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(32) PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(255),
    password_hash TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_login TIMESTAMP
);

-- Create table for user moods if it doesn't exist
CREATE TABLE IF NOT EXISTS user_moods (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(32) NOT NULL,
    animation_id VARCHAR(32) NOT NULL,
    mood VARCHAR(20) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (animation_id) REFERENCES animations(id) ON DELETE CASCADE
);

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_animations_id ON animations(id);
CREATE INDEX IF NOT EXISTS idx_animations_created_at ON animations(created_at);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_last_login ON users(last_login);

CREATE INDEX IF NOT EXISTS idx_user_moods_user_id ON user_moods(user_id);
CREATE INDEX IF NOT EXISTS idx_user_moods_animation_id ON user_moods(animation_id);
CREATE INDEX IF NOT EXISTS idx_user_moods_created_at ON user_moods(created_at);
CREATE INDEX IF NOT EXISTS idx_user_moods_mood ON user_moods(mood);

-- Add a unique constraint to prevent duplicate mood entries
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_moods_unique_user_animation ON user_moods(user_id, animation_id);

-- Add comments on tables and columns for documentation
COMMENT ON TABLE animations IS 'Stores p5.js animation codes with unique IDs';
COMMENT ON COLUMN animations.id IS 'Unique identifier for the animation';
COMMENT ON COLUMN animations.code IS 'The p5.js animation code';
COMMENT ON COLUMN animations.description IS 'Optional description of the animation';
COMMENT ON COLUMN animations.created_at IS 'Timestamp when the animation was created';

COMMENT ON TABLE users IS 'Stores user account information';
COMMENT ON COLUMN users.id IS 'Unique identifier for the user';
COMMENT ON COLUMN users.email IS 'User email address (must be unique)';
COMMENT ON COLUMN users.username IS 'Optional username for display purposes';
COMMENT ON COLUMN users.password_hash IS 'Bcrypt hashed password';
COMMENT ON COLUMN users.created_at IS 'Timestamp when the user account was created';
COMMENT ON COLUMN users.last_login IS 'Timestamp of the last successful login';

COMMENT ON TABLE user_moods IS 'Stores user mood reactions to animations';
COMMENT ON COLUMN user_moods.id IS 'Unique identifier for the mood entry';
COMMENT ON COLUMN user_moods.user_id IS 'Reference to the user who provided the mood';
COMMENT ON COLUMN user_moods.animation_id IS 'Reference to the animation being rated';
COMMENT ON COLUMN user_moods.mood IS 'The mood value (much worse, worse, same, better, much better)';
COMMENT ON COLUMN user_moods.created_at IS 'Timestamp when the mood was recorded';

-- Create mood_statistics view for aggregating mood data
CREATE OR REPLACE VIEW mood_statistics AS
SELECT 
    animation_id,
    COUNT(*) as total_ratings,
    COUNT(CASE WHEN mood = 'much worse' THEN 1 END) as much_worse_count,
    COUNT(CASE WHEN mood = 'worse' THEN 1 END) as worse_count,
    COUNT(CASE WHEN mood = 'same' THEN 1 END) as same_count,
    COUNT(CASE WHEN mood = 'better' THEN 1 END) as better_count,
    COUNT(CASE WHEN mood = 'much better' THEN 1 END) as much_better_count
FROM 
    user_moods
GROUP BY 
    animation_id;

-- Create a materialized view for more efficient mood statistics access
CREATE MATERIALIZED VIEW IF NOT EXISTS mood_statistics_materialized AS
SELECT 
    animation_id,
    COUNT(*) as total_ratings,
    COUNT(CASE WHEN mood = 'much worse' THEN 1 END) as much_worse_count,
    COUNT(CASE WHEN mood = 'worse' THEN 1 END) as worse_count,
    COUNT(CASE WHEN mood = 'same' THEN 1 END) as same_count,
    COUNT(CASE WHEN mood = 'better' THEN 1 END) as better_count,
    COUNT(CASE WHEN mood = 'much better' THEN 1 END) as much_better_count,
    (COUNT(CASE WHEN mood = 'better' THEN 1 END) + COUNT(CASE WHEN mood = 'much better' THEN 1 END) * 2) -
    (COUNT(CASE WHEN mood = 'worse' THEN 1 END) + COUNT(CASE WHEN mood = 'much worse' THEN 1 END) * 2) as mood_score
FROM 
    user_moods
GROUP BY 
    animation_id;

-- Create index on the materialized view
CREATE INDEX IF NOT EXISTS idx_mood_statistics_mat_score ON mood_statistics_materialized(mood_score DESC);
CREATE INDEX IF NOT EXISTS idx_mood_statistics_mat_total ON mood_statistics_materialized(total_ratings DESC);

-- Add any triggers for automation
CREATE OR REPLACE FUNCTION update_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.created_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create function to update the last_login timestamp
CREATE OR REPLACE FUNCTION update_last_login()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE users SET last_login = NOW() WHERE id = NEW.id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create function to refresh mood statistics after mood updates
CREATE OR REPLACE FUNCTION refresh_mood_statistics()
RETURNS TRIGGER AS $$
BEGIN
    REFRESH MATERIALIZED VIEW CONCURRENTLY mood_statistics_materialized;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Create triggers to auto-update timestamps
DO $$
BEGIN
    -- Animation timestamp trigger
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'update_animations_timestamp') THEN
        CREATE TRIGGER update_animations_timestamp
        BEFORE UPDATE ON animations
        FOR EACH ROW
        EXECUTE FUNCTION update_timestamp();
    END IF;
    
    -- User moods trigger for statistics refresh
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'refresh_mood_statistics_trigger') THEN
        CREATE TRIGGER refresh_mood_statistics_trigger
        AFTER INSERT OR UPDATE OR DELETE ON user_moods
        FOR EACH STATEMENT
        EXECUTE FUNCTION refresh_mood_statistics();
    END IF;
END$$;

-- Add last_login column to users table if it doesn't exist
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_schema = 'public' AND table_name = 'users' AND column_name = 'last_login'
    ) THEN
        ALTER TABLE users ADD COLUMN last_login TIMESTAMP;
        RAISE NOTICE 'Added last_login column to users table';
    END IF;
END
$$; 