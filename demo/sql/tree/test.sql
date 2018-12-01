-- ENV "带空格的 时间" '2018-00-00 00:00:00'

-- REF Y4 '2018-00-00 00:00:00'
SELECT year(now()) as Y4;

-- STR '2018-00-00 00:00:00' $y4_table   #重新定义，以使SQL语法正确。非加引号规则
CREATE TABLE tx_parcel_$y4_table LIKE tx_parcel;
-- 替换后
CREATE TABLE tx_parcel_2018 LIKE tx_parcel;

-- STR COL[1] $COL1  #直接定义。
-- STR "`COL[]` = VAL[]" "logno = -99009"  #直接定义，脱壳，加引号，模式展开。
-- REF VAL[1] '占位值'
-- REF id 990001
SELECT * FROM tx_parcel WHERE create_time = '2018-11-23 12:34:56';

INSERT INTO tx_parcel (`$COL1`) VALUES ('占位值');
-- 替换后
INSERT INTO tx_parcel (`id`) VALUES ('占位值');

UPDATE tx_parcel SET logno = -99009 WHERE id=990001;
-- 替换后
UPDATE tx_parcel SET `id` = VAL[1] ,`create_time` = VAL[2] /*循环加下去，逗号分割*/ WHERE id=990001;
