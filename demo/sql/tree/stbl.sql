-- 先创建01-10，共10个表
-- SEQ tx_parcel_%02d[1,10] tx_parcel_##create
CREATE TABLE IF NOT EXISTS `tx_parcel_##create` like `tx_parcel`;
-- RUN FOR tx_parcel_##create
INSERT IGNORE `tx_parcel_##create` SELECT * FROM `tx_parcel` limit 1;
-- OUT FOR tx_parcel_##create
CREATE TABLE IF NOT EXISTS `tx_parcel_##create` like `tx_parcel`;


-- TBL tx_parcel_\d+ tx_parcel_##select
-- REF id 'tx_parcel.id'  #提取 id，作为'tx_parcel.id'节点
-- STR VAL[] 'tx_parcel.VALS'
SELECT * FROM `tx_parcel_##select` limit 1;

-- OUT FOR 'tx_parcel.VALS'
REPLACE INTO `tx_parcel_##select` VALUES ('tx_parcel.VALS');

-- RUN FOR 'tx_parcel.id' # 需要使用 RUN FOR，否则会按顺序立即执行。
DELETE FROM `tx_parcel_##select` where id = 'tx_parcel.id';

-- TBL .*\$log  any$log
DELETE FROM `any$log` where create_time < now() - interval 1 year ;

-- TBL tx_parcel_\d+  parcel_split
DROP TABLE IF EXISTS `parcel_split`;
-- OUT FOR parcel_split
DROP TABLE IF EXISTS `parcel_split`;
