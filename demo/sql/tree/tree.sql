-- ENV DATE_FROM 'ENV_DATE_FROM'  #定义环境变量
-- STR 'ENV_DATE_FROM' $DATE_FROM #重定义，静态替换

-- REF id 'tx_parcel.id'  #提取 id，作为'tx_parcel.id'节点
-- REF recver_id 'tx_parcel.recver_id' # 999777前缀，0001第一个SELECT，002，第二个REF
-- REF track_num 'tx_parcel.track_num' #提取 id，作为'tx_parcel.track_num'节点
-- REF `中文字段` 'tx_parcel.chinese-404' #假设存在，不存在且没引用不报错。
-- STR VAL[] 'tx_parcel.VALS'
SELECT * FROM tx_parcel WHERE create_time <= 'ENV_DATE_FROM';

-- OUT FOR 'tx_parcel.id'
REPLACE INTO tx_parcel VALUES ('tx_parcel.VALS');

-- RUN FOR 'tx_parcel.id' # 需要使用 RUN FOR，否则会按顺序立即执行。
DELETE FROM tx_parcel where id = 'tx_parcel.id';


-- REF id 'tx_track.id' #提取id，作为'tx_track.id'节点，父节点为'tx_parcel.track_num'
-- STR 'tx_parcel.track_num' $TRK
-- STR VAL[] 'tx_track.VALS'
SELECT * FROM tx_track WHERE track_num = 'tx_parcel.track_num';

-- OUT FOR 'tx_track.id'
REPLACE INTO tx_track VALUES ('tx_track.VALS');

-- RUN END 'tx_track.id'
DELETE FROM tx_track where id = 'tx_track.id';


-- REF id 'tx_parcel_event.id' #提取id，作为'tx_parcel_event.id'节点，父节点为'tx_parcel.id'
-- STR VAL[] 'tx_parcel_event.VALS'
SELECT * FROM tx_parcel_event WHERE parcel_id = 'tx_parcel.id';

-- OUT FOR 'tx_parcel_event.id'
INSERT INTO tx_parcel_event VALUES ('tx_parcel_event.VALS')
  ON DUPLICATE KEY UPDATE modify_time = 'ENV_DATE_FROM';

-- RUN END 'tx_parcel_event.id'
DELETE FROM tx_parcel_event where parcel_id = 'tx_parcel_event.id';


-- REF id 'tx_receiver.id'
-- STR `COL[]` `$COLX_9997770004002` # 加引号规则，建议使用SQL合规字符
-- STR VAL[,] 'tx_receiver.VALS'
SELECT * FROM tx_receiver WHERE id = 'tx_parcel.recver_id';

-- OUT FOR 'tx_receiver.id'
REPLACE INTO tx_receiver ($COLX_9997770004002) VALUES ('tx_receiver.VALS');

-- RUN END 'tx_receiver.id'
DELETE FROM tx_receiver where id = 'tx_receiver.id';


-- RUN END 'tx_parcel_event.id'
REPLACE INTO sys_hot_separation VALUES ('tx_parcel_event', 'tx_parcel_event.id', now()); -- 单行注释

-- RUN END 'tx_track.id'
REPLACE INTO sys_hot_separation VALUES ('tx_track', /*内嵌多行注释*/ 'tx_track.id', now());

-- RUN END 'tx_parcel.id'
REPLACE INTO sys_hot_separation VALUES ('tx_parcel', 'tx_parcel.id', now());

-- RUN END 'tx_parcel.id' #存在'tx_parcel.id'节点时执行，即'tx_parcel.id'不为空
DELETE FROM tx_parcel$log WHERE create_time <= 'ENV_DATE_FROM';