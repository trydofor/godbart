-- VAR VER v2019010302
SELECT MAX(version) as VER FROM sys_schema_version WHERE version = 2019010302;
-- RUN NOT v2019010302
-- STR tbl `tx_parcel_#` 为分表更新
SELECT tbl FROM (
  SELECT 'tx_parcel_0' AS tbl  UNION ALL
  SELECT 'tx_parcel_1' UNION ALL
  SELECT 'tx_parcel_2' UNION ALL
  SELECT 'tx_parcel_3') TMP;

-- RUN NOT v2019010302
ALTER TABLE `tx_parcel_#` ADD CONSTRAINT uk_track_num UNIQUE (is_deleted, track_num);
-- RUN NOT v2019010302
REPLACE INTO sys_schema_version (version, created) VALUES(2019010302, NOW());