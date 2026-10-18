-- name: ListZones :many
SELECT * FROM zones ORDER BY code;

-- name: GetZoneByID :one
SELECT * FROM zones WHERE id = $1;

-- name: CreateZone :one
INSERT INTO zones (id, code, name, created_at, updated_at, version)
VALUES ($1, $2, $3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1)
RETURNING *;

-- name: ListStates :many
SELECT * FROM states ORDER BY name;

-- name: ListStatesByZone :many
SELECT * FROM states WHERE zone_id = $1 ORDER BY name;

-- name: GetStateByID :one
SELECT * FROM states WHERE id = $1;

-- name: GetStateByCode :one
SELECT * FROM states WHERE code = $1;

-- name: CreateState :one
INSERT INTO states (id, zone_id, code, name, is_fct, created_at, updated_at, version)
VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1)
RETURNING *;

-- name: ListLGAsByState :many
SELECT * FROM lgas WHERE state_id = $1 ORDER BY name;

-- name: GetLGAByID :one
SELECT * FROM lgas WHERE id = $1;

-- name: CreateLGA :one
INSERT INTO lgas (id, state_id, name, is_area_council, source_reference, revision_date, created_at, updated_at, version)
VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1)
RETURNING *;

-- name: GetCapitalCityByState :one
SELECT * FROM capital_cities WHERE state_id = $1 LIMIT 1;

-- name: CreateCapitalCity :one
INSERT INTO capital_cities (id, state_id, name, created_at, updated_at, version)
VALUES ($1, $2, $3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1)
RETURNING *;

-- name: CountZones :one
SELECT COUNT(*) FROM zones;

-- name: CountStates :one
SELECT COUNT(*) FROM states;

-- name: CountLGAs :one
SELECT COUNT(*) FROM lgas;
