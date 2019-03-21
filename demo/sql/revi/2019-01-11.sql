-- -------------------------------------------
SELECT max(version) FROM sys_schema_version;

-- TBL tx_parcel(\$log)? `tx_parcel#`
ALTER TABLE `tx_parcel#` DROP COLUMN `shelf_time`;

REPLACE INTO sys_schema_version (version, created) VALUES( 2019011101, NOW());

-- -------------------------------------------
SELECT max(version) FROM sys_schema_version;

update sys_schema_version set created = now();

REPLACE INTO sys_schema_version (version, created) VALUES( 2019030601, NOW());

-- -------------------------------------------
SELECT max(version) FROM sys_schema_version;

update sys_schema_version set created = now();

REPLACE INTO sys_schema_version (version, created) VALUES( 2019030701, NOW());

