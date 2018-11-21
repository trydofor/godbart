
DROP TABLE IF EXISTS `tx_parcel$log`;
CREATE TABLE `tx_parcel$log` AS SELECT * FROM `tx_parcel` WHERE 1=0;
ALTER TABLE `tx_parcel$log` ADD COLUMN `_id` INT(11) NOT NULL AUTO_INCREMENT, ADD PRIMARY KEY (`_id`);
ALTER TABLE `tx_parcel$log` ADD COLUMN `_du` INT(11) NULL ;
ALTER TABLE `tx_parcel$log` ADD COLUMN `_dt` DATETIME NULL ;

DROP TRIGGER IF EXISTS `tx_parcel$log$bu`;
DELIMITER $$
CREATE TRIGGER `tx_parcel$log$bu` BEFORE UPDATE ON `tx_parcel`
FOR EACH ROW BEGIN
  insert into `tx_parcel$log` select *, null, 1, now() from `tx_parcel` where id= OLD.id;
END $$
DELIMITER ;

DROP TRIGGER IF EXISTS `tx_parcel$log$bd`;
DELIMITER $$
CREATE TRIGGER `tx_parcel$log$bd` BEFORE DELETE ON `tx_parcel`
FOR EACH ROW BEGIN
  insert into `tx_parcel$log` select *, null, 2, now() from `tx_parcel` where id= OLD.id;
END $$
DELIMITER ;