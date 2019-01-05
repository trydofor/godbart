-- ENV "带空格的 时间" '2018-00-00 00:00:00'
-- ENV DATE_FROM '2018-11-23 12:34:56'
-- ENV ``SELECT YEAR(NOW())`` '2018-00-01 00:00:00'

-- STR USER built_env_user # 直接定义
-- STR HOST built_env_host # 直接定义
-- STR DATE built_env_date # 直接定义

-- STR '2018-00-01 00:00:00' $y4_table   #重新定义，以使SQL语法正确。非加引号规则
DROP TABLE IF EXISTS `tx_parcel_$y4_table`;

CREATE TABLE `tx_parcel_$y4_table` LIKE tx_parcel;
-- 替换后
-- CREATE TABLE tx_parcel_2018 LIKE tx_parcel;

-- STR VAL[1] 990001  #直接定义。
-- STR "`COL[]` = VAL[]" "logno = -99009"  #直接定义，脱壳，加引号，模式展开。
-- REF VAL[,\t] '多值占位值'
-- STR `COL[]` $COLX
SELECT * FROM tx_parcel WHERE create_time > '2018-11-23 12:34:56' LIMIT 2;

-- OUT FOR 990001
REPLACE INTO tx_parcel ($COLX) VALUES ('多值占位值');

-- 替换后
-- REPLACE INTO tx_parcel (`id`) VALUES ('多值占位值');
UPDATE tx_parcel SET logno = -99009 WHERE id = 990001;
-- 替换后
-- UPDATE tx_parcel SET `id` = VAL[1] ,`create_time` = VAL[2] /*循环加下去，逗号分割*/ WHERE id=990001;

-- RUN END 990001 # 在src上执行
-- OUT END 990001 # 也在 dst上执行
INSERT IGNORE INTO sys_hot_separation VALUES ('tx_parcel', 990001, NOW());


-- REF max_id 'tx_item_no.max_id'
select null as max_id;

-- RUN FOR 'tx_item_no.max_id'
replace into sys_hot_separation values ('tx_item_no', 'tx_item_no.max_id', now());