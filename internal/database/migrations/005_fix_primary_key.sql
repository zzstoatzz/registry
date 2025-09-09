-- Migration to fix server ID consistency across versions
-- Problem: Multiple versions of the same server were getting different IDs
-- Solution: Use record_id as primary key, server_id for consistency across versions

-- Check if the new schema is already applied
DO $$
BEGIN
    -- If server_id column already exists, this migration was already run
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'servers' AND column_name = 'server_id') THEN
        RAISE NOTICE 'Migration 005 already applied, skipping...';
        RETURN;
    END IF;

    -- Step 1: Add the new server_id column
    ALTER TABLE servers ADD COLUMN server_id UUID;

    -- Step 2: Add the new record_id column
    ALTER TABLE servers ADD COLUMN record_id VARCHAR(255);

    -- Step 3: Populate server_id with consolidated IDs
    -- For servers with the same name, use the ID from the earliest published version
    WITH earliest_versions AS (
        SELECT 
            value->>'name' as name,
            MIN(value->'_meta'->'io.modelcontextprotocol.registry/official'->>'published_at') as earliest_published,
            value->'_meta'->'io.modelcontextprotocol.registry/official'->>'id' as id
        FROM servers
        GROUP BY value->>'name', value->'_meta'->'io.modelcontextprotocol.registry/official'->>'id'
    ),
    server_id_mapping AS (
        SELECT DISTINCT ON (name)
            name,
            id::UUID as consolidated_id
        FROM earliest_versions
        ORDER BY name, earliest_published
    )
    UPDATE servers
    SET server_id = mapping.consolidated_id
    FROM server_id_mapping mapping
    WHERE servers.value->>'name' = mapping.name;

    -- Step 4: Create record_id as combination of server_id and version
    UPDATE servers 
    SET record_id = server_id::text || '_' || (value->>'version');

    -- Step 5: Drop the old primary key constraint
    ALTER TABLE servers DROP CONSTRAINT servers_pkey;

    -- Step 6: Add new primary key on record_id
    ALTER TABLE servers ADD CONSTRAINT servers_pkey PRIMARY KEY (record_id);

    -- Step 7: Drop the old id column to avoid conflicts
    ALTER TABLE servers DROP COLUMN id;

    -- Step 8: Make server_id NOT NULL
    ALTER TABLE servers ALTER COLUMN server_id SET NOT NULL;

    -- Step 9: Add index on server_id for lookups by server
    CREATE INDEX idx_servers_server_id ON servers(server_id);

    -- Step 10: Update the JSONB value to reflect the consolidated server_id
    UPDATE servers 
    SET value = jsonb_set(
        value, 
        '{_meta,io.modelcontextprotocol.registry/official,id}', 
        to_jsonb(server_id::text)
    );

    RAISE NOTICE 'Migration 005 completed successfully';
END;
$$;