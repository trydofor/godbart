# godbart - go-db-art


```
  |^^^^^^|    /god-bark/ 是一个go写的
  |      |    基于SQL的数据库运维命令行
  | (o)(o)    □ 数据库和数据的版本管理
  @      _)   □ 比较表，索引，触发器
   | ,___|    □ 以业务`数据树`迁移数据
   |   /      □ 纯SQL写`数据树`的配置
```

使用场景的前置要求

 * 数据库主键具有分布式特征。
 * 每组SQL间，要有空行分割。

开发和测试环境，ubuntu 16.04 

 * Go 1.11.2
 * MySQL (5.7.23)

## 1.版本管理

每次`schema`或数据的更新，都需要有版本管理。通常，使用一个sequence表，记录版本号。
并且我们只考虑升级不考虑降级。如果出现需要降级的情况时，建议以负相补丁形式进行升级。

```mysql
# 创建version表
CREATE TABLE `sys_schema_version` (
  `version` BIGINT NOT NULL COMMENT '版本号',
  `created` DATETIME NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
```

版本号要求必须唯一且递增.下例是有版本的SQL，版本规则为日期+序号：`yyyymmdd###`。
版本管理的SQL书写有特别的格式，每个版本块，必须被版本查询和更新的SQL包围。
因此，一个有版本的SQL文件，首个单值SELECT和尾行的非SELECT，视为版本查询和更新。

```mysql
-- 版本查询
SELECT max(version) FROM sys_schema_version;

ALTER TABLE `tx_outer_trknum`
  ADD COLUMN `label_file` VARCHAR(200) DEFAULT NULL COMMENT '面单文件位置' AFTER `trknum`;
ALTER TABLE `tx_outer_trknum$log`
  ADD COLUMN `label_file` VARCHAR(200) DEFAULT NULL COMMENT '面单文件位置' AFTER `trknum`;

-- 版本更新
REPLACE INTO sys_schema_version (version, created) VALUES( 2018022801, NOW());
```

## 2.结构对比

对两个不同的数据库。
 * 比较表，索引，触发器的差异。
 * 生成表与索引，trigger的DDL

## 3.数据迁移

数据活性，不同业务场景有不同的定义，比如按日期，按ID范围，甚至ID取余。
本功能只支持静态分库，即对既有数据，在执行前已预知数据范围和目标数据库。
因为动态分库，通常有业务代码负责，而不会沦落到"SQL+数据维护"的层面。
此外，要求表的主键具有分布式主键特质（不支持单表自增型，破坏数据关系）

```mysql
# 建立分库有关的表
CREATE TABLE `sys_hot_separation` (
  `table_name` VARCHAR(100) NOT NULL COMMENT '表名',
  `checked_id` BIGINT(20) NOT NULL COMMENT '检查过的最大ID',
  `checked_tm` DATETIME NOT NULL COMMENT '上次检查的时间',
  PRIMARY KEY (`table_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
```

设置分库时，数据迁移规则必须预先可知，如下脚本根据上次迁移ID，迁移10棵以tx_parcel为根的`数据树`。
并每迁移一棵树，就会执行一次`END`，已完成此树的标记和清理工作。
```mysql
-- REF checked_id 990001  数据树根节点
SELECT checked_id FROM sys_hot_separation WHERE table_name = 'tx_parcel';

-- REF id 990002  一级树节点990002，父节点是 990001
-- REF track_num 'TRK0001'  提取结果中的id和track_num作为变量，形成数据树
SELECT * FROM tx_parcel WHERE id > 990001 LIMIT 10;

-- REF id 990003  二级树节点990003，父节点是 TRK0001
SELECT * FROM tx_track WHERE track_num = 'TRK0001';

-- REF id 990004 二级树节点990004，父节点是 990002
SELECT * FROM tx_parcel_event WHERE parcel_id = 990002;

-- RUN FOR 990002 每棵990002树节点完成时，执行此语句
REPLACE INTO sys_hot_separation(table_name, checked_id, checked_tm) VALUES 
 ('tx_parcel_event', 990004, now())
,('tx_track', 990003, now())
,('tx_parcel', 990002, now());
```

## 4.变量说明

`数据数data-tree`配置，使用SQL的单行注释定义`变量para`和`占位hold`，然后在执行时进行替换。
这样的好处能够保留SQL的可读性和执行能力，每个SQL段直接要留有空行，否则会作为一组SQL同时执行。
`数据树`按SQL从上至下关系提取，并以此顺序导入其他数据库，所以如有外键约束，需要注意插入顺序。

 * `指令` 固定值，分大小写，只有ENV|REF|RUN|STR
 * `变量`和`占位`要求相同，但都区分大小写。
 * ```[^ \t'"`]+``` 连续非引号空白 ```(['"`]).+\1```成对引号括起来的字符串（贪婪）
 * `占位`必须当前SQL中全局唯一，不与其他字符串混淆，以准确替换，确定数据关系（`RUN`不计入）。

### 4.1.环境变量 ENV

`ENV` 通过 `-e MY_ENV="my val"`传入，没有`=`表示时，使用系统变量，如 `-e PATH`。
内置以下变量`USER`，`HOST`，`DATE`，表示用户，主机和日时(yyyy-mm-dd HH:MM:ss)

如下SQL，定义环境变量`DATE_FROM`，其占位符`'2018-11-23 12:34:56'` ，
需要通过系统环境变量获得，如果不存在则会报错。
```mysql
-- ENV DATE_FROM '2018-11-23 12:34:56'
SELECT * FROM tx_parcel WHERE create_time = '2018-11-23 12:34:56';
```
假设运行时 `DATE_FROM`的值为`'2018-01-01 00:00:00'`，那么上述SQL执行时为，
是采用PreparedStatement的动态形式，以防止SQL转义或注入。

```mysql
-- 实际参数为'2018-01-01 00:00:00'
SELECT * FROM tx_parcel WHERE create_time = ? 
```

### 4.2.结果引用 REF

`REF` 采用处理方式仍是PreparedStatement形式，在结果集每次循环中生成。
多个`REF`会产生多个分叉点，进而形成不同的子树，父子关系以深度优先。

注意，只有以`SELECT * ` 语句会产生分叉点，形成数据树，完整的迁移数据。
而`SELECT ID,TRACK_NUM`，只能作为变量引用，而不能形成迁移数据树。

如下SQL，定义了结果集的引用 `id`和`track_num`变量，和他们对应的SQL占位符。
其中，`id`和`track_num`，都是`tx_parcel`的结果集中，用来描述数据树。

```mysql
-- ENV DATE_FROM '2018-11-23 12:34:56'
-- REF id 1234567890
-- REF track_num 'TRK1234567890'
SELECT * FROM tx_parcel WHERE create_time = '2018-11-23 12:34:56';

SELECT * FROM tx_track WHERE track_num = 'TRK1234567890';

SELECT * FROM tx_parcel_event WHERE parcel_id = 1234567890;
```

### 4.3.条件执行 RUN

执行条件由`REF`或`ENV`定义，目前支持的条件和含义如下，

 * `FOR` 表示`REF`所在节点为根，每棵树结束时执行
 * `END` 表示`REF`所在节点为根，所有树结束时执行。
 * `HAS` 表示`ENV`或`REF`对应的变量值存在时执行。数值大于0，布尔true，非NULL，其他转为字符串后非空。

多个`FOR`和`END`时，是`OR`关系。存在`HAS`时，以`AND`处理。

```mysql
-- RUN END 1234567890
REPLACE INTO sys_hot_separation(table_name, checked_id, checked_tm) VALUES 
('tx_parcel', 1234567890, now());
```

### 4.4.静态替换 STR

`ENV`和`REF`都采用的是动态形式，但仍有部分情况需要静态替换字符串，此时使用`STR`。
它可以把其他占位符重新命名，然后在执行的时候，使用静态字符串替换的方式。

```mysql
-- REF Y4 992018
SELECT year(now()) as Y4;

-- STR 992018 y4_table
CREATE TABLE tx_parcel_y4_table LIKE tx_parcel;
-- 替换后
CREATE TABLE tx_parcel_2018 LIKE tx_parcel;
```
