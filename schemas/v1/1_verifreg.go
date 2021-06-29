package v1

// Schema version 1 adds verified registry actor state tracking

func init() {
	patches.Register(
		1,
		`
	CREATE TYPE {{ .SchemaName | default "public"}}.verified_registry_event_type AS ENUM (
		'ADDED',
		'REMOVED'
	);

	CREATE TABLE IF NOT EXISTS {{ .SchemaName | default "public"}}.verified_registry_verifier_events (
		"height"		bigint  NOT NULL,
		"state_root"	text    NOT NULL,
		"address"		text 	NOT NULL,
		"event" 		{{ .SchemaName | default "public"}}.verified_registry_event_type NOT NULL,
		PRIMARY KEY ("height", "state_root", "address", "event")
	);
	COMMENT ON TABLE {{ .SchemaName | default "public"}}.verified_registry_verifier_events IS 'Verifier events on-chain per each verifier added or removed.';
	COMMENT ON COLUMN {{ .SchemaName | default "public"}}.verified_registry_verifier_events.height IS 'Epoch at which this verifier event occurred.';
	COMMENT ON COLUMN {{ .SchemaName | default "public"}}.verified_registry_verifier_events.state_root IS 'CID of the parent state root at this epoch.';
	COMMENT ON COLUMN {{ .SchemaName | default "public"}}.verified_registry_verifier_events.address IS 'Address of verifier this event applies to.';
	COMMENT ON COLUMN {{ .SchemaName | default "public"}}.verified_registry_verifier_events.event IS 'Name of the event that occurred.';

	CREATE TABLE IF NOT EXISTS {{ .SchemaName | default "public"}}.verified_registry_client_events (
		"height"		bigint  NOT NULL,
		"state_root"	text    NOT NULL,
		"address"		text 	NOT NULL,
		"event" 		{{ .SchemaName | default "public"}}.verified_registry_event_type NOT NULL,
		PRIMARY KEY ("height", "state_root", "address", "event")
	);
	COMMENT ON TABLE {{ .SchemaName | default "public"}}.verified_registry_client_events IS 'Verifier events on-chain per each verifier client added or removed.';
	COMMENT ON COLUMN {{ .SchemaName | default "public"}}.verified_registry_client_events.height IS 'Epoch at which this verified client event occurred.';
	COMMENT ON COLUMN {{ .SchemaName | default "public"}}.verified_registry_client_events.state_root IS 'CID of the parent state root at this epoch.';
	COMMENT ON COLUMN {{ .SchemaName | default "public"}}.verified_registry_client_events.address IS 'Address of verified client this event applies to.';
	COMMENT ON COLUMN {{ .SchemaName | default "public"}}.verified_registry_client_events.event IS 'Name of the event that occurred.';

	CREATE TABLE IF NOT EXISTS {{ .SchemaName | default "public"}}.verified_registry_verifiers (
		"height"		bigint  NOT NULL,
		"state_root"	text    NOT NULL,
		"address"		text 	NOT NULL,
		"data_cap" 		numeric NOT NULL,

		PRIMARY KEY ("height", "state_root", "address")
	);
	COMMENT ON TABLE {{ .SchemaName | default "public"}}.verified_registry_verifiers IS 'Verifier on-chain per each verifier state change.';
	COMMENT ON COLUMN {{ .SchemaName | default "public"}}.verified_registry_verifiers.height IS 'Epoch at which this verifiers state changed.';
	COMMENT ON COLUMN {{ .SchemaName | default "public"}}.verified_registry_verifiers.state_root IS 'CID of the parent state root at this epoch.';
	COMMENT ON COLUMN {{ .SchemaName | default "public"}}.verified_registry_verifiers.address IS 'Address of verifier this state change applies to.';
	COMMENT ON COLUMN {{ .SchemaName | default "public"}}.verified_registry_verifiers.data_cap IS 'DataCap of verifier at this state change.';

	CREATE TABLE IF NOT EXISTS {{ .SchemaName | default "public"}}.verified_registry_verified_clients (
		"height"		bigint  NOT NULL,
		"state_root"	text    NOT NULL,
		"address"		text 	NOT NULL,

		"data_cap" 		numeric NOT NULL,

		PRIMARY KEY ("height", "state_root", "address")
	);
	COMMENT ON TABLE {{ .SchemaName | default "public"}}.verified_registry_verified_clients IS 'Verifier on-chain per each verified client state change.';
	COMMENT ON COLUMN {{ .SchemaName | default "public"}}.verified_registry_verified_clients.height IS 'Epoch at which this verified client state changed.';
	COMMENT ON COLUMN {{ .SchemaName | default "public"}}.verified_registry_verified_clients.state_root IS 'CID of the parent state root at this epoch.';
	COMMENT ON COLUMN {{ .SchemaName | default "public"}}.verified_registry_verified_clients.address IS 'Address of verified client this state change applies to.';
	COMMENT ON COLUMN {{ .SchemaName | default "public"}}.verified_registry_verified_clients.data_cap IS 'DataCap of verified client at this state change.';
`)
}
