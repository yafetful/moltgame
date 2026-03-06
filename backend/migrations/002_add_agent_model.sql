-- Add model field to agents table (e.g. "claude-sonnet-4", "gpt-4o", "gemini-2.5-flash")
ALTER TABLE agents ADD COLUMN model VARCHAR(100) DEFAULT '';
