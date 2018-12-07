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

## 1.执行脚本 Exec

在不同的db上，批量执行脚本。脚本中支持环境变量，结果集引用，条件执行等。

```bash
# 执行 demo/sql/init/的`*.sql`和`*.xsql`
./godbart exec \
 -c godbart.toml \
 -d prd_main -d prd_2018 \
 -x .sql -x .xsql \
 demo/sql/init/
```

其中，`exec` 命令，会把输入的文件或路径以SQL执行。

 * `-c` 必填，配置文件位置。
 * `-d` 必填，目标数据库，可以指定多个。
 * `-x` 选填，SQL文件后缀，不区分大小写。
 * `--agree` 选填，风险自负，真正执行。

## 2.版本管理 Revi

每次`schema`或数据的更新，都需要有版本管理。通常，使用一个sequence表，记录版本号。
并且我们只考虑升级不考虑降级。如果出现需要降级的情况时，建议以负相补丁形式进行升级。

```bash
# 执行 demo/sql/revi/*.sql，具体SQL写法参考此目录的文件
./godbart revi \
 -c godbart.toml \
 -d prd_main -d prd_2018 \
 -r 2018111701 \
 -m 'v_[0-9]{10,}'
 -x .sql -x .xsql \
 demo/sql/revi/
```

其中，`revi` 命令，会把输入的文件或路径的SQL进行版本切分。

 * `-c` 必填，配置文件位置。
 * `-d` 必填，目标数据库，可以指定多个。
 * `-r` 必填，执行到的版本号。
 * `-m` 选填，更新版本语句的Revision规则，默认10位以上数字。
 * `-x` 选填，SQL文件后缀，不区分大小写。
 * `--agree` 选填，风险自负，真正执行。

`版本号`要求，
 * 必须唯一且递增
 * 可以当做字符串比较大小，如日期+序号：`yyyymmdd###`。
 * 具有相同的格式，可以用正则匹配


版本管理的SQL书写有特别的格式，每个版本块，必须被`版本查询`和`版本更新`的SQL包围。
因此，SQL文件中，首个单值SELECT和最尾的Execute，视为版本查询和更新的SQL。

作为参数传入的版本文件，需要增序，与内置的版本顺序一致。

```mysql
-- 创建version表 # 此时没有版本查询，但在之前，因此会被执行
CREATE TABLE `sys_schema_version` (
  `version` BIGINT NOT NULL COMMENT '版本号',
  `created` DATETIME NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

-- 版本查询
SELECT max(version) FROM sys_schema_version;

ALTER TABLE `tx_outer_trknum`
  ADD COLUMN `label_file` VARCHAR(200) DEFAULT NULL COMMENT '面单文件位置' AFTER `trknum`;
ALTER TABLE `tx_outer_trknum$log`
  ADD COLUMN `label_file` VARCHAR(200) DEFAULT NULL COMMENT '面单文件位置' AFTER `trknum`;

-- 版本更新
REPLACE INTO sys_schema_version (version, created) VALUES( 2018022801, NOW());
```

## 3.结构对比 Diff

对一个或多个数据库的表结构，能够生成表字段加索引，trigger的创建DDL。

过程信息使用log输出stderr(`2`)。结果信息使用stdout(`1`)输出。因此可以分离信息。

 * `> main-2018-diff-out.log` 结果直接保存文件，控制台不输出。
 * `2> main-2018-diff-err.log` 过程保存文件，控制台不输出。
 * `&> main-2018-diff-all.log` 全部保存文件，控制台不输出。
 * `| tee main-2018-diff-out.log` 结果保存文件，且控制台输出。
 * `2>&1| tee >(grep -vE '^[0-9]{4}' > main-2018-diff-out.log)` 同上。
 * `2>&1| tee main-2018-diff-all.log` 全部保存文件，且控制台输出。

```bash
# 对表名，字段，索引，触发器都进行比较，并保存结果到 main-2018-diff.log
./godbart diff \
 -c godbart.toml \
 -s prd_main \
 -d prd_2018 \
 -k detail \
 'tx_.*' \
> main-2018-diff-out.log
```

 * `-s` 为左侧比较相，可以零或一。
 * `-d` 为右侧比较相，可以零或多。
 * `-k` 为比较类型，支持create的ddl，detail的表明细，tbname的仅表名。
    - `create` 生成多库的创建DDL(table&index，trigger)
    - `detail` 分别对比`-s`和多个`-d` 间的表明细(column, index,trigger)
    - `tbname` 分别对比`-s`和多个`-d` 间的表名差异
 * `--agree` 选填，风险自负，真正执行。

参数为需要对比的表的名字的正则表达式。如果参数为空，表示所有表。
`-s`和`-d`，必须指定一个。只有一个时，仅打印该库，多个时才进行比较。

`表明细Detail`的内容格式中，用`>`表示只有左侧存在，`<`表示只有右侧存在。


## 4.数据迁移 Tree

不建议一次转移大量数据，可能引起网络超时或内存不够存放临时数据。

```bash
# 对表名，字段，索引，触发器都进行比较，并保存结果到 main-2018-diff.log
./godbart tree \
 -c godbart.toml \
 -s prd_main \
 -d prd_2018 \
 -x .sql -x .xsql \
 -e "DATE_FROM=2018-11-23 12:34:56"
 demo/sql/tree/tree.sql
 > main-tree-out.log
```

数据活性，不同业务场景有不同的定义，比如按日期，按ID范围，甚至ID取余。
本功能只支持静态分库，即对既有数据，在执行前已预知数据范围和目标数据库。
因为动态分库，通常有业务代码负责，而不会沦落到"SQL+数据维护"的层面。
此外，要求表的主键具有分布式主键特质（不支持单表自增型，破坏数据关系）

数据树(DataTree)的核心是`占位`。唯一的占位符，可以准确描述数据关系，
自定义占位，可以满足基本的SQL语法。占位必须先声明再使用，以区别普通文字。

```mysql
-- 建立分库有关的表
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
-- REF checked_id 990001  #数据树根节点
SELECT checked_id FROM sys_hot_separation WHERE table_name = 'tx_parcel';

-- REF id 990002  #一级树节点990002，父节点是 990001
-- REF track_num 'TRK0001'  #提取结果中的id和track_num作为变量，形成数据树
SELECT * FROM tx_parcel WHERE id > 990001 LIMIT 10;

-- REF id 990003  #二级树节点990003，父节点是 TRK0001
SELECT * FROM tx_track WHERE track_num = 'TRK0001';

-- REF id 990004 #二级树节点990004，父节点是 990002
SELECT * FROM tx_parcel_event WHERE parcel_id = 990002;

-- RUN FOR 990002 #每棵990002树节点完成时，执行此语句
REPLACE INTO sys_hot_separation(table_name, checked_id, checked_tm) VALUES 
 ('tx_parcel_event', 990004, now())
,('tx_track', 990003, now())
,('tx_parcel', 990002, now());
```

## 5.指令变量

`数据数data-tree`配置，使用SQL的单行注释定义`变量para`和`占位hold`，然后在执行时进行替换。
这样的好处能够保留SQL的可读性和执行能力，每个SQL段直接要留有空行，否则会作为一组SQL同时执行。
`数据树`按SQL从上至下关系提取，并以此顺序导入其他数据库，所以如有外键约束，需要注意插入顺序。

 * `指令` 固定值，不区分大小写，当前只支持，`ENV` `REF` `STR` `RUN` `OUT`
 * `引号`包括，单引号`'`，双引号`"`，反单引号`` ` ``。
 * `空白`指英文空格`0x20`和制表符`\t`
 * `变量`和`占位`要求相同，都区分大小写。
    - ```[^ \t'"`]+``` 连续非引号空白
    - ```(['"`])[^\1]+\1```成对引号括起来的字符串(非贪婪)
    - 使用时，最外层的成对引号会被去掉（`STR`特殊）。
 * `占位`，在SQL语句中的占位符。
    - 定义`占位` 必须当前SQL中全局唯一，不与其他字符串混淆，以准确替换，确定数据关系。
    - `ENV` `REF` `STR` 为定义指令，`RUN` `OUT` 为使用指令。
    - 尽量使用SQL中合法数据格式，没必要自找麻烦，比如没必要的引号，特殊字符等。
    - 使用时，保留所有引号。
    
 变量必须先声明再使用，否则无法正确识别占位符。

### 5.1.环境变量 ENV

`ENV` 全局有效，通过 `-e MY_ENV="my val"`传入，只有Key表示使用系统变量，如 `-e PATH`。
内置以下变量，可以通过命令行覆盖。
 - `USER`，当前用户
 - `HOST`，主机名
 - `DATE`，当前日时(yyyy-mm-dd HH:MM:ss)
 - `ENV-CHECK-RULE`，不存在时，如何处理，默认是`ERROR`：报错；`EMPTY`：置空；

如下SQL，定义环境变量`DATE_FROM`，其占位符`'2018-11-23 12:34:56'` ，
需要通过系统环境变量获得，如果不存在则会报错。

假设运行时 `DATE_FROM`的值为`'2018-01-01 00:00:00'`，那么上述SQL执行时为，
是采用PreparedStatement的动态形式，以防止SQL转义或注入。

```mysql
-- ENV DATE_FROM '2018-11-23 12:34:56'
SELECT * FROM tx_parcel WHERE create_time = '2018-11-23 12:34:56';

-- 运行时替换，比如实际参数为'2018-01-01 00:00:00'
SELECT * FROM tx_parcel WHERE create_time = ?
```

### 5.2.结果引用 REF

`REF` 也是PreparedStatement形式替换，只对所在结果集的每天记录产生循环。
多个`REF`会产生多个分叉点，进而形成不同的子树。

当子语句，只依赖一个`REF`的`占位`(如9900397)时，相当于`RUN FOR 9900397`，
因为时唯一关系，所以可以省略`RUN/OUT`，两者时等价的。

当子语句，会依赖多个`REF`的`占位`(如9900398,9900399)时，为了避免歧义，
必须使用 `RUN/OUT`精确描述。

如下SQL，定义了结果集的引用 `id`和`track_num`变量，和他们对应的SQL占位符。
其中，`id`和`track_num`，都是`tx_parcel`的结果集中，用来描述数据树。

`变量`可以被引号包围，以用来更好的使用空白或中文字段等情况（见，脱引号处理）。

```mysql
-- ENV DATE_FROM '2018-11-23 12:34:56'
-- REF `id` 1234567890  #假设id需要反单引号处理
-- REF track_num 'TRK1234567890'
SELECT * FROM tx_parcel WHERE create_time = '2018-11-23 12:34:56';

SELECT * FROM tx_track WHERE track_num = 'TRK1234567890';

SELECT * FROM tx_parcel_event WHERE parcel_id = 1234567890;
```

较为特殊的是，系统为`SELECT *` 内定了结果集引用，以便可以构建insert和update语句。

 * `COL[]` 表示所有列名，会展开为 `id`,`name`,等（可以转义）
 * `VAL[]` 表示结果的值，会展开为 `?`占位符和对应值。
 * `COL[1]` 表示获得第1个列名
 * `VAL[2]` 表示获得第2个值
 
 其中，角标从1开始。引用为数组时，`[]`内要制定分隔符，默认时`,`。
 即，`COL[]`和`COL[,]`相同。存在多个分隔符时，只取第一个非空的。
 分隔符，不能用数字，因为做角标；不能用`[]`，因为你懂的。

### 5.3.静态替换 STR

`STR`与`ENV`和`REF`不同，采用的是静态替换字符串。可以对结果集或占位符使用。
`STR` 可以用 `ENV`和`REF` 的`变量`，也以用`占位`重新定义。

`脱引号`处理，当`变量`和`占位`具有相同的引号规则，则同时脱去最外的一层引号。
此规则只对`STR`有效，因为其变量部分，可以重定义其他有引号的`占位`。

`加引号`处理，如果`脱引号`后，`变量`仍有引号包围，那么替换时会增加包围的引号。
因为此规则的存在，当`变量`必须带引号时，需要先`REF`在`STR`才能正确的处理。

`模式展开`，`变量`中有多个多值内容`COL[*]`或`VAL[*]`时，可以展开。
`模式展开`限制比较多，因为不想简单的事情搞复杂了。

 - 首先脱引号处理。
 - 只支持直接定义，不支持重新定义。
 - 除了`COL[*]`和`VAL[*]`外，都作为静态字符串处理，不会深度展开。
 - 只支持`\t`,`\n`，`\\`转移。
 - `COL[*]`部分，使用静态替换。
 - `VAL[*]`部分，仍然使用PreparedStatement形式执行。
 

```mysql
-- REF Y4 '2018-00-00 00:00:00'
SELECT year(now()) as Y4;

-- STR '2018-00-00 00:00:00' $y4_table   #重新定义，以使SQL语法正确。非加引号规则
CREATE TABLE tx_parcel_$y4_table LIKE tx_parcel;
-- 替换后
CREATE TABLE tx_parcel_2018 LIKE tx_parcel;

-- STR COL[1] $COL1  #直接定义。
-- STR "`COL[]` = VAL[]" "logno = -99009"  #直接定义，脱引号，加引号，模式展开。
-- REF VAL[1] '占位值'
-- REF id 990001
SELECT * FROM tx_parcel WHERE create_time = '2018-11-23 12:34:56';

INSERT INTO tx_parcel (`$COL1`) VALUES ('占位值');
-- 替换后
INSERT INTO tx_parcel (`id`) VALUES ('占位值');

UPDATE tx_parcel SET logno = -99009 WHERE id=990001;
-- 替换后
UPDATE tx_parcel SET `id` = ? ,`create_time` = ? /*循环加下去，逗号分割*/ WHERE id=990001;
```

### 5.4.条件执行 RUN

执行条件由`REF`或`ENV`定义，只对后续的第一条语句有效。目前支持的条件和含义如下，

 * `FOR` 表示`REF`所在节点为根，每棵树结束时执行。等效于Hold依赖关系
 * `END` 表示`REF`所在节点为根，所有树结束时执行。
 * `HAS` 表示`占位`变量有值时执行。有值指，数值大于0，布尔true，非NULL，其他转为字符串后非空。
 * `NOT` 与`HAS`相反。

多个`FOR`和`END`时，是`OR`关系。`HAS`/`NOT`与其他是`AND`处理。

与`REF`的`HOLD`确定单父关系不同，`RUN`可以确定多父关系。并且，当有`RUN`确立父级关系时忽略`REF`的。

```mysql
-- RUN END 1234567890
REPLACE INTO sys_hot_separation(table_name, checked_id, checked_tm) VALUES 
('tx_parcel', 1234567890, now());
```

### 5.5.输出执行 OUT

与条件执行 `RUN` 一样的定义，只是不在源DB上执行，而是在目标DB上执行。

注意，在有`定义Def`结果集（`REF`或`STR`直接定义）的语句上，不能使用`OUT`。
因为一个`占位`在运行时存在多值的情况，从而导致语义混乱或执行时错误。

```mysql
-- ENV DATE_FROM '2018-11-23 12:34:56'
-- REF id 1234567890
SELECT * FROM tx_parcel WHERE create_time = '2018-11-23 12:34:56';

-- OUT FOR 1234567890
REPLACE INTO tx_parcel VALUES(1234567890);
```

## 6.测试手册

使用工程中/demo/sql下的SQL进行所有功能的演示和测试。以下是准备工作，你必须都懂。
注意，所有对数据库有写操作的命令，都需要增加`--agree`才会执行，否则仅输出预计结果。

### 6.1. 获得执行文件

```bash
# 下载，也可以直接下载release文件，直接到unzip步骤
# https://github.com/trydofor/godbart/releases

git clone https://github.com/trydofor/godbart.git
cd godbart

# 编译
chmod +x build.sh
./build.sh

ls -l release 
# 解压对应系统的执行文件，默认linux
unzip release/godbart-linux-amd64.zip

# 得到 godbart 程序
```

### 6.2. 修改数据源配置

修改`godbart.toml`中的数据库用户名，密码，主机，端口等

```bash
# 你的用户是 yourname
sed -i 's/trydofor:/yourname:/g' godbart.toml
# 你的密码是 yourpass
sed -i 's/:moilioncircle@/:yourpass@/g' godbart.toml
# 你的ip是 127.0.0.9
sed -i 's/(127.0.0.1:/(127.0.0.9:/g' godbart.toml
# 你的端口是 13306
sed -i 's/:3306)/:13306)/g' godbart.toml
```

### 6.3. 创建数据库

```bash
# 存在一个可使用的数据库，如一般都有的test
./godbart exec \
 -c godbart.toml \
 -d lcl_test \
 --agree \
 demo/sql/diff/reset.sql
 
 # 或用 mysql命令，创新数据库
 cat demo/sql/diff/reset.sql \
 | mysql -h127.0.0.1 \
 -utrydofor \
 -P3306 \
 -p"moilioncircle"
```

### 6.4. Exec 执行脚本

使用 exec 执行init中的脚本初始化 prd_main 数据库。

```bash
./godbart exec \
 -c godbart.toml \
 -d prd_main \
 --agree \
 demo/sql/init/
```


### 6.5. Revi 版本控制

执行revi中的脚本使 prd_2018 更新到 2018111103 版本（只有基础结构）。
以为 prd_main 版本号比 2018111103 所以会跳过小版本的更新。

```bash
./godbart revi \
 -c godbart.toml \
 -d prd_main \
 -d prd_2018 \
 -r 2018111103 \
 --agree \
 demo/sql/revi/
```

### 6.6. Diff 结构差异

使用 diff 执行比较 prd_main 与 prd_2018, dev_main 差异。

```bash
# 查看 prd_main dev_main的表名差异（默认）
./godbart diff \
 -c godbart.toml \
 -s prd_main \
 -d dev_main
 
# 显示 tx_parcel表在prd_main上的创建语句
./godbart diff \
 -c godbart.toml \
 -s prd_main \
 -k create \
  tx_parcel \
  | tee /tmp/ddl-tx_parcel-main.sql

# 比较 tx_parcel 在prd_main和prd_2018详细差异
./godbart diff \
 -c godbart.toml \
 -s prd_main \
 -d prd_2018 \
 -k detail \
  tx_parcel \
  | tee /tmp/diff-tx_parcel-main-2018.sql
```

### 6.7. Tree 迁移数据

按数据的业务关系，形成数据数，整棵迁移。

此例中，因为危险操作比较多，务必先分离脚本，人工确认。
脚本可以在对应的数据库上执行，需要注意二进制或字符串转义。

对于复杂数据类型，可`--agree`，在程序中以动态数据来执行。

```bash
# 危险动作，先保持日志查看
./godbart tree \
 -c godbart.toml \
 -s prd_main \
 -d prd_2018 \
 -e "DATE_FROM=2018-11-23 12:34:56" \
 demo/sql/tree/tree.sql \
 | tee /tmp/tree-main-2018.log
 
# 过滤出目标库脚本
cat /tmp/tree-main-2018.log \
| grep -E '^--' | cut -c 4- | grep -v  "SRC" \
| tee /tmp/tree-main-2018_out.log

```
