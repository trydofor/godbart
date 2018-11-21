-- ENV DATE_FROM '2018-11-23 12:34:56'  定义环境变量
-- REF id 99002  提取 id，作为99002节点
-- REF track_num 'TRK0001' 提取 id，作为TRK0001节点
-- REF `中文字段` '中文引用'
SELECT * FROM tx_parcel WHERE create_time <= '2018-11-23 12:34:56';

-- REF id 990003 提取id，作为990003节点，父节点为TRK0001
SELECT * FROM tx_track WHERE track_num = 'TRK0001';

-- REF id 990004 提取id，作为990004节点，父节点为990002
SELECT * FROM tx_parcel_event WHERE parcel_id = 990002;

-- RUN FOR 990002 每次完成节点990002时执行
REPLACE INTO sys_hot_separation(table_name, checked_id, checked_tm) VALUES
 ('tx_parcel_event', 990004, now()) -- 单行注释
,('tx_track', 990003, now())  /*内嵌多行注释*/
,('tx_parcel', 990002, now());

-- RUN HAS 990002 存在990002节点时执行，即990002不为空
DELETE FROM tx_parcel$log WHERE create_time <= '2018-11-23 12:34:56';