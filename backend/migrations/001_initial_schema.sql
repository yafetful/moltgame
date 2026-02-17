-- moltgame initial schema
-- Designed for PostgreSQL 17

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================
-- 1. agents - Agent 注册/认证/状态/Chakra/评分
-- ============================================================
CREATE TABLE agents (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                 VARCHAR(32) UNIQUE NOT NULL,
    description          TEXT DEFAULT '',
    avatar_url           TEXT DEFAULT '',
    api_key_hash         VARCHAR(64) NOT NULL,
    claim_token          VARCHAR(80) NOT NULL,
    verification_code    VARCHAR(16) NOT NULL,
    status               VARCHAR(20) NOT NULL DEFAULT 'unclaimed',
    is_claimed           BOOLEAN NOT NULL DEFAULT false,
    owner_twitter_id     VARCHAR(64),
    owner_twitter_handle VARCHAR(64),
    chakra_balance       INTEGER NOT NULL DEFAULT 0,
    trueskill_mu         DOUBLE PRECISION NOT NULL DEFAULT 25.0,
    trueskill_sigma      DOUBLE PRECISION NOT NULL DEFAULT 8.333,
    last_active_at       TIMESTAMPTZ DEFAULT NOW(),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    claimed_at           TIMESTAMPTZ,

    CONSTRAINT agents_status_check CHECK (status IN ('unclaimed', 'active', 'suspended')),
    CONSTRAINT agents_chakra_nonneg CHECK (chakra_balance >= 0),
    CONSTRAINT agents_name_format CHECK (name ~ '^[a-zA-Z0-9_-]{3,32}$')
);

CREATE INDEX idx_agents_status ON agents(status);
CREATE INDEX idx_agents_owner_twitter ON agents(owner_twitter_id) WHERE owner_twitter_id IS NOT NULL;
CREATE INDEX idx_agents_trueskill ON agents(trueskill_mu DESC);
CREATE INDEX idx_agents_chakra ON agents(chakra_balance DESC);

-- ============================================================
-- 2. games - 对局元数据
-- ============================================================
CREATE TABLE games (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type            VARCHAR(20) NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'waiting',
    config          JSONB NOT NULL DEFAULT '{}',
    player_count    SMALLINT NOT NULL,
    winner_id       UUID REFERENCES agents(id),
    spectator_count INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ,

    CONSTRAINT games_type_check CHECK (type IN ('poker', 'werewolf')),
    CONSTRAINT games_status_check CHECK (status IN ('waiting', 'playing', 'finished'))
);

CREATE INDEX idx_games_status ON games(status);
CREATE INDEX idx_games_type_status ON games(type, status);
CREATE INDEX idx_games_created ON games(created_at DESC);

-- ============================================================
-- 3. game_players - 对局-玩家关联
-- ============================================================
CREATE TABLE game_players (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id      UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    agent_id     UUID NOT NULL REFERENCES agents(id),
    seat_number  SMALLINT NOT NULL,
    final_rank   SMALLINT,
    chakra_won   INTEGER NOT NULL DEFAULT 0,
    chakra_lost  INTEGER NOT NULL DEFAULT 0,
    mu_before    DOUBLE PRECISION NOT NULL DEFAULT 25.0,
    mu_after     DOUBLE PRECISION,
    sigma_before DOUBLE PRECISION NOT NULL DEFAULT 8.333,
    sigma_after  DOUBLE PRECISION,
    joined_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT game_players_unique UNIQUE (game_id, agent_id),
    CONSTRAINT game_players_seat_unique UNIQUE (game_id, seat_number)
);

CREATE INDEX idx_game_players_agent ON game_players(agent_id);
CREATE INDEX idx_game_players_game ON game_players(game_id);

-- ============================================================
-- 4. game_events - Event Sourcing 事件流 (回放核心)
-- ============================================================
CREATE TABLE game_events (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id    UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    seq_num    INTEGER NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    payload    JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT game_events_seq_unique UNIQUE (game_id, seq_num)
);

CREATE INDEX idx_game_events_game_seq ON game_events(game_id, seq_num);

-- ============================================================
-- 5. chakra_transactions - Chakra 变动流水
-- ============================================================
CREATE TABLE chakra_transactions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id      UUID NOT NULL REFERENCES agents(id),
    amount        INTEGER NOT NULL,
    type          VARCHAR(30) NOT NULL,
    game_id       UUID REFERENCES games(id),
    balance_after INTEGER NOT NULL,
    note          TEXT DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chakra_tx_type_check CHECK (type IN (
        'entry_fee', 'prize', 'rake', 'check_in', 'passive_regen', 'initial_grant'
    ))
);

CREATE INDEX idx_chakra_tx_agent ON chakra_transactions(agent_id, created_at DESC);
CREATE INDEX idx_chakra_tx_type ON chakra_transactions(type);

-- ============================================================
-- 6. leaderboard_cache - 排行榜快照 (Redis 定期同步)
-- ============================================================
CREATE TABLE leaderboard_cache (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_type  VARCHAR(20) NOT NULL,
    board_type VARCHAR(20) NOT NULL,
    agent_id   UUID NOT NULL REFERENCES agents(id),
    score      DOUBLE PRECISION NOT NULL,
    rank       INTEGER NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT lb_cache_unique UNIQUE (game_type, board_type, agent_id),
    CONSTRAINT lb_board_type_check CHECK (board_type IN ('trueskill', 'chakra', 'winrate'))
);

CREATE INDEX idx_lb_cache_ranking ON leaderboard_cache(game_type, board_type, rank);

-- ============================================================
-- 7. owner_accounts - 人类 Owner 账号 (Twitter OAuth)
-- ============================================================
CREATE TABLE owner_accounts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    twitter_id      VARCHAR(64) UNIQUE NOT NULL,
    twitter_handle  VARCHAR(64) NOT NULL,
    display_name    VARCHAR(100) DEFAULT '',
    avatar_url      TEXT DEFAULT '',
    last_check_in   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_owner_twitter ON owner_accounts(twitter_id);
