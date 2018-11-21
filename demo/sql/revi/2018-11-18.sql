-- -------------------------------------------
SELECT max(version) FROM sys_schema_version;

ALTER TABLE `tx_parcel`
  ADD INDEX `ix_user_id` (`user_id` ASC),
  ADD INDEX `ix_recver_id` (`recver_id`),
  ADD UNIQUE `uq_trknum` (`track_num` ASC);

ALTER TABLE `tx_track`
  ADD INDEX `ix_user_id` (`user_id` ASC),
  ADD INDEX `ix_parcel_id` (`parcel_id` ASC),
  ADD UNIQUE `uq_trknum` (`track_num` ASC);

ALTER TABLE `tx_parcel_event`
  ADD INDEX `ix_user_id` (`user_id` ASC),
  ADD INDEX `ix_parcel_id` (`parcel_id` ASC);

ALTER TABLE `tx_receiver`
  ADD INDEX `ix_user_id` (`user_id` ASC),
  ADD INDEX `ix_name` (`name` ASC),
  ADD INDEX `ix_addr` (`province`,`city`,`district`,`address1` ASC),
  ADD INDEX `ix_hash` (`hash` ASC),
  ADD FULLTEXT `ft_phone` (`phone` ASC);

REPLACE INTO sys_schema_version (version, created) VALUES( 2018111801, NOW());
