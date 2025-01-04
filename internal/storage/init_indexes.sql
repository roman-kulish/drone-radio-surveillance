-- Essential indexes for joins and basic querying

-- For telemetry join
CREATE INDEX IF NOT EXISTS idx_samples_telemetry ON samples(telemetry_id)
    WHERE telemetry_id IS NOT NULL;

-- Telemetry table
CREATE INDEX IF NOT EXISTS idx_telemetry_session_time ON telemetry(session_id);

-- For session-wide frequency and time ranges + aggregates
CREATE INDEX IF NOT EXISTS idx_samples_session_time_freq ON samples(session_id, timestamp, frequency);

-- For aggregates
CREATE INDEX IF NOT EXISTS idx_samples_session_freq_time ON samples(session_id, frequency, timestamp);