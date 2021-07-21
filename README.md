# godbart - go-db-art


```
  |^^^^^^|    /god-bart/是一个go写的
  |      |    基于SQL的RDBMS运维CLI
  | (o)(o)    □ 多库执行SQL，DB版本管理
  @      _)   □ 比较结构差异，生成原始DDL
   | ,___|    □ 提取业务逻辑关联的`数据树`
   |   /      □ 纯SQL做配置，注释做关联
```

使用场景和前置要求，

 * DBA维护多库，一个SQL在多库上执行。
 * 支持分表，多表的更新和版本管理。
 * 生成某库某表的创建SQL（表&索引，触发器）。
 * 对比多库多表的结构差异（表，列，索引，触发器）。
 * 多库的版本管理，按指定版本更新。
 * 提取`数据树`，保存为CSV/JSON文件。
 * 数据归档，从A库迁移`数据树`到B库。
 * 主键有分布式特征，无自增型。
 * SQL语句，必须有结束符，如`;`，否则认为是一组。
 * 当前只适配了MySql，可自行实现PG版。
 * 正则Regexp为go标准库`re2`，有些高级功能不支持。

`数据树(DataTree)` 指一堆有业务逻辑关联的树状或图状的数据。
比如`demo/init/2.data.sql`中的关系，存在以下多个`1:N`关系。
```
|-(TOP)-收件人(tx_receiver)
|      |-(1:N)-包裹(tx_parcel)
|      |      |-(1:N)-物流信息(tx_track)
|      |      |-(1:N)-包裹事件(tx_parcel_event)
|      |      |-(1:N)-历史变更(tx_parcel$log)
```
就可以形成以`收件人`为根的树，或从`包裹`为根的树。
对于非单继承（多个父节点）的数据结构，有多重循环时会存在问题。

## 1. 场景举例

以下是开发和测试环境，得益于GoLang的优势，理论上应该跨平台。

 * ubuntu 16.04/ mac 10.15.7 
 * Go 1.11.2 / 1.16
 * MySQL (5.7.23)
 
下列各命令的参数，大部分时通用的，所以举例中不重复介绍各参数。

### 1.1. 执行脚本 Exec

在不同的db上，纯粹的批量执行SQL。

```bash
# 执行 demo/sql/init/的`*.sql`和`*.xsql`
./godbart exec \
 -c godbart.toml \
 -d prd_main \
 -d prd_2018 \
 -x .sql -x .xsql \
 -l trace \
 demo/sql/init/
```

其中，`exec` 命令，会把输入的文件或路径，分成SQL组执行。

 * `-c` 必填，配置文件位置。
 * `-d` 必填，目标数据库，可以指定多个。
 * `-x` 选填，SQL文件后缀，不区分大小写。
 * `-l` 选填，通过修改输出级别，调整信息量。
 * `--agree` 选填，风险自负，真正执行。
 
 在分表上执行，参考`revi`说明。

### 1.2. 版本管理 Revi

健康的数据库需要有版本管理。通常，有一个版本信息表，用来识别和对比版本号。
`Revi`只考虑Up不考虑Down。如果需要Down时，以`逆向补丁`形式进行Up。

```bash
# 执行 demo/sql/revi/*.sql，具体SQL写法参考此目录的文件
./godbart revi \
 -c godbart.toml \
 -d prd_main \
 -d prd_2018 \
 -r 2018111701 \
 -m '[0-9a-z]{10,}'
 -x .sql -x .xsql \
 demo/sql/revi/
```

其中，`revi` 命令，会把输入的文件或路径的SQL进行按版本号分组。

 * `-c` 必填，配置文件位置。
 * `-d` 必填，目标数据库，可以指定多个。
 * `-r` 必填，执行到的版本号。
 * `-m` 选填，版本更新语句中版本号的正则，默认10位以上数字。
 * `-q` 选填，查询版本语句的前缀，`SELECT` 不区分大小写。
 * `-x` 选填，SQL文件后缀，不区分大小写。
 * `--agree` 选填，风险自负，真正执行。

`版本号`要求，
 * 必须全局唯一且递增，但不要求连续。
 * 能以字符串方式比较大小，如日期+序号：`yyyymmdd###`。
 * 具有可以用正则匹配提取的固定格式。


具有版本管理的SQL要求，必须被`版本查询`和`版本更新`的SQL包围。
因此，SQL文件中，首个SELECT和最尾的Execute，视为版本查询和更新的SQL。

作为参数传入的版本文件，内含版本号需要递增，否则报错（程序只检查，不排序）。

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

### 1.3. 分表版本管理 Revi

当存在分表的情况下，可以按序号建表，或者根据规则更新已存在的表。
更多关于`指令`可以参考`指令变量`说明，及`tree`应用实例。

```mysql
-- SEQ tx_test_%02d[1,10] tx_test_##
CREATE TABLE `tx_test_##` (
  `id` BIGINT NOT NULL COMMENT 'id',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;


-- TBL tx_outer_trknum.* `tx_outer_trknum`
ALTER TABLE `tx_outer_trknum`
  ADD COLUMN `label_file` VARCHAR(200) DEFAULT NULL COMMENT '面单文件位置' AFTER `trknum`;
```

上述SQL会完成以下两种操作。

 * 创建 tx_test_01,...,tx_test_20，一共20张表
 * 更新 tx_outer_trknum 和 tx_outer_trknum$log表

### 1.4. 结构对比 Diff

用来对比结构差异，支持table&index，trigger。

对比结果中，用`>`表示只有左侧存在，`<`表示只有右侧存在。
过程信息以log输出。结果信息fmt输出，可通过`SHELL`特性分离信息。

```bash
# 对表名，字段，索引，触发器都进行比较，并保存结果到 main-2018-diff-out.log
./godbart diff \
 -c godbart.toml \
 -s prd_main \
 -d prd_2018 \
 -d dev_main \
 -t tbl,trg \
 'tx_.*' \
| tee main-2018-diff-out.log
```

 * `-s` 左侧比较相，必须指定。
 * `-d` 右侧比较相，可以零或多。
 * `-t` 比较类型，支持以下三种，默认`tbl`，多值时用逗号分割。
    - `tbl` 表明细(column, index)
    - `trg` trigger
    - `sum` 仅显示表名差异

参数为需要对比的表的名字的正则表达式。如果参数为空，表示所有表。
正则会默认进行全匹配，等同于`^$`的效果。

当只有一个库时，不做比较，而是打印该库，多个时才进行比较。

### 1.5. 生成脚本 Show

生成一些常用的DDL，如创建table, trigger，更复杂的history历史表。

```bash
./godbart show \
 -c godbart.toml \
 -s prd_main \
 -t tbl,trg \
 'tx_[^$]+' \
| tee prd_main-show-out.log
```
模板在`godbart.toml`中的`sqltemplet`里配置，`key`就是`-t` 参数，多个时用`,`分割。
模板使用的`变量`全都存在时，输出模板，全都不存在时不输出，其他则报错。

因为不支持否定环视，所有已不包含`$`去除`$log`表。

系统内置了以下`变量`，不想使用`${}`不可以省略，包含数组的模板会循环输出。

 * ${TABLE_NAME}   string, 当前table名
 * ${TABLE_DDL}    string, 当前table的DDL
 * ${TRIGGER_NAME} []string, 当前table的trigger名
 * ${TRIGGER_DDL}  []string, 当前table的trigger的DDL
 * ${COLUMNS_BASE} string, 当前table的所有列的基本信息(名字和类型)。
 * ${COLUMNS_FULL} string, 当前table的所有列的全部信息(同创建时，创建DDL必须一行一列，否则解析可能错误)。

### 1.6. 结构同步 Sync

同步多库间的表结构，目前只支持空表创建。此场景一般出现在初始化一个新数据库的时候。
因为数据库版本管理不会造成很大差异，如果存在差异，且有数据的情况下，人工介入更好。

对于小表，提供数据同步。而多实例，大表，建议使用DBA的方式同步，性能更好。

注意，对于DBA，可以使用`mysqldump -d`来导出表结构。

```bash
./godbart sync \
 -c godbart.toml \
 -s prd_main \
 -d prd_2018 \
 -t tbl,trg \
 'tx_.*'
```

 * `-s` 左侧比较相，可以零或一。
 * `-d` 右侧比较相，可以一或多。
 * `-t` 创建类型，支持以下三种，默认`tbl`。
    - `tbl` 只创建表和索引
    - `trg` 只创建trigger
    - `row` 标准insert语法，并忽略重复，不如DBA脚本猛烈，适合小数据。
 * `--agree` 选填，风险自负，真正执行。

参数为需要对比的表的名字的正则表达式。如果参数为空，表示所有表。

### 1.7. 数据迁移 Tree

不建议一次转移大量数据，有概率碰到网络超时或内存紧张。

```bash
# 把数据从main迁移到2018库，结果保存到main-tree-out.log
./godbart tree \
 -c godbart.toml \
 -s prd_main \
 -d prd_2018 \
 -x .sql -x .xsql \
 -e "DATE_FROM=2018-11-23 12:34:56" \
 demo/sql/tree/tree.sql
 > main-tree-out.log

# 静态分析上面的datatree语法结构。
./godbart sqlx \
 -c godbart.toml \
 -e "DATE_FROM=2018-01-01 00:00:00" \
 demo/sql/tree/tree.sql \
 | tee /tmp/sqlx-tree.log
```

不同业务场景对`数据活性`有不同的定义，比如日期，按ID范围等。
`Tree`命令只支持静态分离数据，即在执行前已预知数据范围和目标数据库。
因为动态分库，通常有业务代码负责，而不会沦落到"SQL+数据维护"的层面。
此外，要求表的主键具有分布式主键特质（自增型主机很糟糕，破坏数据关系）

数据树(DataTree)的核心是`占位`，其具有以下特性。

 * 定义（Def）的唯一性。
 * 可以准确描述数据关系。
 * 可以满足基本的SQL语法。
 * 占位必须先声明再使用，以区别普通字面量。

```mysql
-- 建立分库有关的表
CREATE TABLE `sys_hot_separation` (
  `table_name` VARCHAR(100) NOT NULL COMMENT '表名',
  `checked_id` BIGINT(20) NOT NULL COMMENT '检查过的最大ID',
  `checked_tm` DATETIME NOT NULL COMMENT '上次检查的时间',
  PRIMARY KEY (`table_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
```

分离数据的规则必须预先可知，如下脚本根据历史信息，迁移10棵以tx_parcel为根的`数据树`。
并且每迁移一棵树，就会在源数据库上执行一次`FOR`，用来完成此树的标记和清理工作。

注意：`FOR`时强关系，`REF`是弱关系，两者的关联和区别，见后面章节。

```mysql
-- 不存在则增加默认值
INSERT IGNORE sys_hot_separation VALUES ('tx_parcel',0,now());

-- VAR checked_id 'tx_parcel.checked_id'  #数据树根节点
SELECT checked_id FROM sys_hot_separation WHERE table_name = 'tx_parcel';

-- REF id 'tx_parcel.id'  #一级树节点'tx_parcel.id'，父节点是 'tx_parcel.checked_id'
-- REF track_num 'tx_parcel.track_num'  #提取结果中的id和track_num作为变量，形成数据树
SELECT * FROM tx_parcel WHERE id > 'tx_parcel.checked_id' LIMIT 10;

-- REF id 990003  #二级树节点990003，父节点是 TRK0001
SELECT * FROM tx_track WHERE track_num = 'tx_parcel.track_num';

-- REF id 990004 #二级树节点990004，父节点是 'tx_parcel.id'
SELECT * FROM tx_parcel_event WHERE parcel_id = 'tx_parcel.id';

-- RUN FOR 'tx_parcel.id' #每棵'tx_parcel.id'树节点完成时，执行此语句
REPLACE INTO sys_hot_separation VALUES ('tx_parcel', 'tx_parcel.id', now());
```

### 1.8. 控制端口

对于长时间执行的命令，支持单例和运行时控制（如优雅停止），因此增加了`控制端口`功能。
其监听TCP端口（建议1024以上），当端口号≤0时，表示忽略此功能。
开启`控制端口`时，会在stderr输入`控制密码`，通过`127.0.0.*`登录不需要密码。

 * 单例，检测`控制端口`是否被监听，保证当前主机唯一单例。
 * 控制，通过tcp链接，输入`控制密码`，验证后，执行支持的命令。

全局命令：
 * help - 查看帮助。
 * exit - 关闭当前连接。
 * pass - 生成一个新密码，作废旧密码，新登录有效。
 * info - 查看当前用户和待执行的命令。
 * kill N - 杀掉队列中id=N的任务，N=-1时，清掉全部。
 * `/` 公聊，跟所有登录用户发消息。
 * `/ip:port ` 私聊，指定登录用户发消息。

非全局命令，称作一个`room`，一个room内，改变行为的命令的信息时全员可见的。
只对`Tree`提供了以下命令，可使用不存在的id查看当前运行情况。


 * tree - 显示当前在执行的sqlx的树状结构及ID。
 * stat - 显示当前在执行的信息。
 * stop - 优雅的停止程序(exit 99)，全员可见。
   - stop 直接在当前树结束时停止。
   - stop N 在id=N的树时停止，N<0时等效于stop。
 * wait - 执行等待，kill可继续。长时间停止可能导致数据库连接超时。全员可见。
   - wait 在当前树完成时等待。
   - wait N 在id=N的树时停止，N<0时等效于stop。
   
```bash
# 连接控制端口，非127.0.0.* 登录，需要先输入密码
telnet 127.0.0.1 59062
# 以下为连接成功输入的命令。

info # 查看运行信息
tree # 查看当前执行`数据树`结构
stat # 查看统计情况
wait 0 # 空等待，显示每个执行节点信息。
kill # 清理掉所有任务
stop # 优雅停止当前一棵树的结束
```

## 2. 指令变量

`指令`在SQL的注释中定义，由`指令名`，`变量para`和`占位hold`三部分构成。
`指令`保留SQL的可读性和执行能力，对DBA友好，在运行时进行静态或动态替换。

`数据树`按SQL的自然顺序构建和执行，`占位`必须先声明再使用，否则无法正确识别。
明确语意和增加可读性，`RUN|OUT`存在顺序调整，下文有讲。

`挂树`是指数据数分叉时，寻找父树的动作，当前的规则如下：
 * `RUN|OUT` 属于显示`挂树`，优先级是`10`，支持多父结构
 * `REF|STR`，是隐式挂树，只支持单父，取按行号大者，优先级是`20`。
 * `SEQ|TBL`，同`REF`，优先级时`30`。
 * 按优先级`挂树`，数值越小优先级越高，当高优先级完成后，忽略低优先级。

 * `指令名`是固定值，当前只支持，`ENV|REF|STR|RUN|OUT`
    - `ENV|REF|STR|SEQ|TBL` 等会产生值，为定义（Def）指令。
    - `RUN|OUT` 为行为（Act）指令。
    - `ENV|REF` 对`变量`自动脱去最外层成对的引号。
    - `STR` 有自己的脱引号规则，以进行`模式展开`。
 * `引号`包括，单引号`'`，双引号`"`，反单引号`` ` ``。
 * `空白`指英文空格`0x20`和制表符`\t`
 * `变量`和`占位`要求相同，都区分大小写。
    - ```[^ \t'"`]+``` 连续的不包括引号和空白的字符串。
    - ```(['"`])[^\1]+\1```成对引号括起来的字符串(非贪婪)。
 * `占位`，在SQL语句符合语法的字面量（数字，字符串，语句等）。
    - 必须当前SQL中全局唯一，不与其他字面量混淆，以准确替换，确定数据关系。
    - 尽量使用SQL的合规语法，没必要自找麻烦，比如没必要的引号或特殊字符。
    - 使用时，保留所有引号。
    - 选择`占位`，尽量构造出where条件为false的无公害SQL。

注意：所有包含`空白`的`变量`和`占位`，都需要有`引号`配合

### 2.1. 环境变量 ENV

`ENV`通过 `-e MY_ENV="my val"`从命令行传入，全局有效。
当只有Key时，表示使用系统变量，如 `-e PATH`。

系统内置了以下变量，
 
 - `USER`，当前用户
 - `HOST`，主机名
 - `DATE`，当前日时(yyyy-mm-dd HH:MM:ss)
 - `ENV-CHECK-RULE`，ENV检查规则，默认报错，可用`EMPTY`置空
 - `SRC-DB`，当前执行的源DB（只有Tree，且唯一）；
 - `OUT-DB`，当前执行的目标DB（只有Tree，只有OUT时能确定）；

当`变量`被`1个以上`的`反单引号`包围时，表示此`ENV`通过运行`SQL`获得，
是第一条记录的第一个字段。优点是不会被纳入数据树，缺点是不享受SQL高亮，
不能替换其他占位。注意 STR不支持这么骚气的操作，因为有模式展开。

如下SQL，定义环境变量`DATE_FROM`，其占位符`'2018-11-23 12:34:56'` ，
需要通过系统环境变量获得，如果不存在（默认ERROR）则会报错。

假设运行时 `DATE_FROM`的值为`'2018-01-01 00:00:00'`，那么上述SQL执行时为，
是采用PreparedStatement的动态形式，可避免SQL转义或注入，提高运行时性能。

```mysql
-- ENV ``SELECT NOW();`` sql_now  运行时赋值

-- ENV DATE_FROM '2018-11-23 12:34:56'
SELECT * FROM tx_parcel WHERE create_time = '2018-11-23 12:34:56';

-- 运行时替换，比如实际参数为'2018-01-01 00:00:00'
-- SELECT * FROM tx_parcel WHERE create_time = ?
```

### 2.2. 结果引用 REF

`REF` 也采用PreparedStatement替换，并对所在结果集的每条记录循环。
多个`REF`会产生多个分叉点，进而形成不同的子数据树。

当子语句，只依赖一个`REF`的`占位`(如9900397)时，相当于`RUN FOR 9900397`，
两者在关系上等价的，但执行时机不同，前者在树中，后者在树末。

当子语句，会依赖多个`REF`的`占位`(如9900398,9900399)时，为了避免歧义，
必须使用 `RUN/OUT`精确描述，否则系统会任性选择。

如下SQL，定义了结果集的引用 `id`和`track_num`变量，和他们对应的SQL占位符。
其中，`id`和`track_num`，都是`tx_parcel`的结果集中，用来描述数据树。

```mysql
-- ENV DATE_FROM '2018-11-23 12:34:56'
-- REF `id` 1234567890  #假设id需要反单引号处理
-- REF track_num 'TRK1234567890'
SELECT * FROM tx_parcel WHERE create_time = '2018-11-23 12:34:56';

SELECT * FROM tx_track WHERE track_num = 'TRK1234567890';

SELECT * FROM tx_parcel_event WHERE parcel_id = 1234567890;
```

系统为结果集（SELECT）内定了引用，以便可以多值insert和update语句。

 * `COL[]` 表示所有列名，会展开为 `id`,`name`,等（可以转义）
 * `VAL[]` 表示结果的值，会展开为 `?`占位符和对应值。
 * `COL[1]` 表示获得第1个列名
 * `VAL[2]` 表示获得第2个值
 
 其中，角标从1开始。引用为数组时，在`[]`内指定分隔符，约定如下，
 
 * `COL[]`和`COL[,]`相同，分隔符默认是`,`。
 * 存在多个分隔符时，只取第一个非空的。
 * 不能用数字，因为做角标
 * 不能用`[`或`]`，因为你懂的。
 * 仅支持`\\`，`\t`，`\n`的字符转义。

### 2.3. 变量声明 VAR

同`REF`一样作用于结果集，但不形成树状结构。和`ENV`相比，可以时string之外的SQL类型。

### 2.4. 静态替换 STR

`STR`与`ENV`和`REF`不同，采用的是静态替换字符串。
它可以直接定义（同`REF`和`ENV`），也以重新定义其他动态`占位`使其静态化。

`脱引号`处理，当`变量`和`占位`具有相同的引号规则，会都脱去最外的一层。
此规则只对`STR`有效，因为其变量部分，可以重定义带有引号的`占位`。

`模式展开`，`变量`中有`COL[*]`或`VAL[*]`时，会进行展开，规则如下，

 - 首先脱引号处理。
 - 只支持直接定义，不支持重新定义。
 - 除了`COL[*]`和`VAL[*]`外，都作为字面量处理，不会深度展开。
 - `COL[*]`部分，使用静态替换。
 - `VAL[*]`部分，使用PreparedStatement形式执行。

```mysql
-- REF Y4 '2018-00-00 00:00:00'
SELECT year(now()) as Y4;

-- STR '2018-00-00 00:00:00' $y4_table   #重新定义，以使SQL语法正确。
CREATE TABLE tx_parcel_$y4_table LIKE tx_parcel;
-- 替换后
-- CREATE TABLE tx_parcel_2018 LIKE tx_parcel;

-- STR COL[1] $COL1  #直接定义。
-- STR "`COL[]` = VAL[]" "logno = -99009"  #直接定义，脱引号，模式展开。
-- REF VAL[1] '占位值'
-- REF id 'tx_parcel.checked_id'
SELECT * FROM tx_parcel WHERE create_time = '2018-11-23 12:34:56';

INSERT INTO tx_parcel (`$COL1`) VALUES ('占位值');
-- 替换后
-- INSERT INTO tx_parcel (`id`) VALUES (?);

UPDATE tx_parcel SET logno = -99009 WHERE id='tx_parcel.checked_id';
-- 替换后
-- UPDATE tx_parcel SET `id` = ? ,`create_time` = ? /*循环加下去，逗号分割*/ WHERE id='tx_parcel.checked_id';
```

### 2.5. 整数序列 SEQ

`SEQ`会生成整数序列，只支持静态替换。

 * `参数` 格式为`格式[开始,结束,步长]`，如`tx_test_%02d[1,20]`
 * `格式`为`fmt`的`printf`标准格式
 * `开始`和`结束`都时闭区间，是包含的
 * `步长`可以省略，默认是`1`
 * 注意`空白`
 
`SEQ`会其定义处产生循环，但不产生树。对自身及子树有影响。

详见 `demo/sql/tree/stbl.sql`
 
 ```mysql
 -- SEQ `tx_test_%02d[1,10]` tx_test_## #生成tx_test_01到tx_test_10，共10张表
 CREATE TABLE `tx_test_##` (
   `id` BIGINT NOT NULL COMMENT 'id',
   PRIMARY KEY (`id`)
 ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
```

 
### 2.6. 表名匹配 TBL

`TBL`根据当前库现有表进行匹配，只支持静态替换

 * `参数` 为正则，行为上等同于使用了`^$`，会对表名进行全匹配，不是部分。
 * 也可以使用`\b`或 `^|$`进行界定

规则同`SEQ`，详见 `demo/sql/tree/stbl.sql`。
当在Out执行时，表名为当前数据库内所有表。

```mysql
 -- TBL tx_outer_trknum.* `tx_outer_trknum` # 正则匹配表名。
 ALTER TABLE `tx_outer_trknum`
   ADD COLUMN `label_file` VARCHAR(200) DEFAULT NULL COMMENT '面单文件位置' AFTER `trknum`;
```

### 2.7. 条件执行 RUN

执行条件由`REF`或`ENV`定义，只对所在的语句有效，执行顺序与SQL行顺序有关。

 * `ONE` 以定义`占位`的节点为根，第一棵树时执行。
 * `FOR` 以定义`占位`的节点为根，每棵树时执行，等效于`REF`。
 * `END` 以定义`占位`的节点为根，最后一棵树执行。
 * `HAS` 表示`占位`变量有值时执行该树。`有值`指，
    - 数值大于`0`
    - 布尔`true`
    - 非`NULL`
    - 字符串非空（`“”`）
    - 其他类型强转为字符串后非空。
 * `NOT` 与`HAS`相反。

条件执行，有以下约定关系，

 * 多个`ONE|FOR|END`是`OR`关系。
 * `HAS|NOT`自身或与其他是`AND`关系。
 * `RUN` 可以确定多个父关系，且强于`REF`。
 * `RUN` 在树结束时执行，而`REF`在树中执行。
 * 数据点增序排列，权重为`REF`<`ONE`<`FOR`<`END`，同级时算SQL位置。
 * 增加`ITSELF`占位，表示单独执行，没有任何依赖。

条件执行的例子，参考 `demo/sql/tree/*.sql`

### 2.8. 输出执行 OUT

与条件执行 `RUN` 一样的定义，但不在源DB上执行，而是在目标DB上执行。

注意，在有`定义Def`语句，如`REF`或`SEQ`等，不能使用`OUT`。
因为一个`占位`在运行时存在多值，从而导致语义混乱或执行时麻烦。

```mysql
-- ENV DATE_FROM '2018-11-23 12:34:56'
-- REF id 1234567890
SELECT * FROM tx_parcel WHERE create_time = '2018-11-23 12:34:56';

-- OUT FOR 1234567890
REPLACE INTO tx_parcel VALUES(1234567890);
```

## 3. 测试手册

使用工程中/demo/sql下的SQL进行所有功能的演示和测试。以下是准备工作，你必须都懂。
注意，所有对数据库有写操作的命令，都需要增加`--agree`才会执行，否则仅输出预计结果。

可以分步人工确认，也可以在工程目录中执行`demo/chk/manual.sh`自动确认。
执行之前，需要增加执行权限，`chmod +x `，并设置好mysql连接信息。

### 3.1. 获得执行文件

```bash
### 方法一：下载 ###
# 直接下载release文件，直接到unzip步骤
# https://github.com/trydofor/godbart/releases

### 方法二：编译 ###

git clone https://github.com/trydofor/godbart.git
cd godbart

# 单平台编译
GOOS=linux GOARCH=amd64 go build

# 或全平台发布
chmod +x build.sh
./build.sh

ls -l release 
# 解压对应系统的执行文件，默认linux
unzip release/godbart-linux-amd64.zip

# 得到 godbart 程序
```

### 3.2. 修改数据源配置

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

### 3.3. 创建数据库

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

### 3.4. Exec 执行脚本

使用 exec 执行init中的脚本初始化 prd_main 数据库。

```bash
./godbart exec \
 -c godbart.toml \
 -d prd_main \
 --agree \
 demo/sql/init/
```

### 3.5. Revi 版本控制

执行revi中的脚本使 prd_2018 更新到 2018111103 版本（只有结构没有数据）。
因为prd_main 版本号比 2018111103 所以会跳过小版本的更新。

```bash
./godbart revi \
 -c godbart.toml \
 -d prd_main \
 -d prd_2018 \
 -r 2018111103 \
 --agree \
 demo/sql/revi/
```

### 3.7. Sync 结构同步

复制prd_main表结构到dev_main

```bash
./godbart sync \
 -c godbart.toml \
 -s prd_main \
 -d dev_main \
 -t tbl,trg \
 --agree
 
# 同步小表（表结构版本）
./godbart sync \
 -c godbart.toml \
 -s prd_main \
 -d dev_main \
 -t row \
 --agree \
 sys_schema_version
```

### 3.7. Diff 结构差异

使用 diff 执行比较 prd_main 与 prd_2018, dev_main 差异。

```bash
# 查看 prd_main 与 dev_main的表名差异，sync后完全一致
./godbart diff \
 -c godbart.toml \
 -s prd_main \
 -d dev_main \
 -t tbl,trg
 
# 显示 tx_parcel表在prd_main上的创建语句
./godbart show \
 -c godbart.toml \
 -s prd_main \
 -t tbl,trg \
  tx_parcel \
| tee /tmp/ddl-tx_parcel-main.sql

# 比较 tx_parcel 在prd_main和prd_2018详细差异
./godbart diff \
 -c godbart.toml \
 -s prd_main \
 -d prd_2018 \
 -t tbl,trg \
  tx_parcel \
| tee /tmp/diff-tx_parcel-main-2018.sql
```

### 3.8. SqlX 静态分析

静态分析 DataTree结构。

```bash
./godbart sqlx \
 -c godbart.toml \
 -e "DATE_FROM=2018-01-01 00:00:00" \
 demo/sql/tree/tree.sql \
 | tee /tmp/sqlx-tree.log
```

### 3.9. Tree 保存JSON

把数据，保持成TSV（TAB分割），CSV（逗号分割）和JSON。
此例中，有`脱引号`，`模式展开` 的组合。

```bash
# 危险动作，先保持日志查看。
# 注意SQL以DEBUG输出，用TRACE会没有输出。
./godbart tree \
 -c godbart.toml \
 -s prd_main \
 -e "DATE_FROM=2018-01-01 00:00:00" \
 demo/sql/tree/json.sql \
 | tee /tmp/tree-main-json.log
 
#分离和处理，去掉注释和结束符
cat /tmp/tree-main-json.log \
| grep -E '^--' | grep -vE  "^(-- )+(SRC|OUT)" \
| sed -E 's/^-- |;$//g' \
| tee /tmp/tree-main-json.txt
```
### 3.10. Tree 迁移数据

此例中，因为危险操作比较多，务必先分离脚本，人工确认。
脚本99%可以执行，在二进制或转义字符转换字面量可能有遗漏。

字面量不好描述的类型，可`--agree`，在程序中以动态数据来执行。

```bash
# 危险动作，先保持日志查看
./godbart tree \
 -c godbart.toml \
 -s prd_main \
 -d prd_2018 \
 -e "DATE_FROM=2018-01-01 00:00:00" \
 demo/sql/tree/tree.sql \
2>&1| tee /tmp/tree-main-2018-all.log
 
# 获得全部SQL
cat /tmp/tree-main-2018-all.log \
| grep -vE '^[0-9]{4}/[0-9]{2}|^$' \
| tee /tmp/tree-main-2018-all.sql

# 获得源库SQL
cat /tmp/tree-main-2018-all.sql \
| grep -E '^[^-]|-- SRC' \
| tee /tmp/tree-main-2018-src.sql

# 获得目标库SQL
cat /tmp/tree-main-2018-all.sql \
| grep -E '^--' | cut -c 4- | grep -v  "-- SRC" \
| tee /tmp/tree-main-2018-out.sql

# 直接执行
./godbart tree \
 -c godbart.toml \
 -s prd_main \
 -d prd_2018 \
 -e "DATE_FROM=2018-01-01 00:00:00" \
 --agree \
 demo/sql/tree/tree.sql \
2>&1| tee /tmp/tree-main-2018-all.log
```

## 4. 实用小技巧

数据的日常处理，会有很多技巧，能提高数据意识，培养直觉。

### 4.1. SHELL分离信息

过程信息以log在stderr(`2`)输出。结果信息以stdout(`1`)输出，
`1`和`2`是描述符，`>`表重定向，`&`表合并，组合起来可分离信息。

 * `> main-2018-diff-out.log` 结果直接保存文件，控制台不输出。
 * `2> main-2018-diff-err.log` 过程保存文件，控制台不输出。
 * `&> main-2018-diff-all.log` 全部保存文件，控制台不输出。
 * `| tee main-2018-diff-out.log` 结果保存文件，且控制台输出。
 * `2>&1| tee >(grep -vE '^[0-9]{4}' > main-2018-diff-out.log)` 同上。
 * `2>&1| tee main-2018-diff-all.log` 全部保存文件，且控制台输出。

### 4.2. 按数据量排序

查询所有表的记录数，对于单表300万的数据，进行按树分离或清理。

```mysql
-- 按记录数排序，同时查看磁盘空间
SELECT 
    TABLE_NAME,
    TABLE_ROWS,
    FLOOR(DATA_LENGTH  / 1048576) AS DATA_M,
    FLOOR(INDEX_LENGTH / 1048576) AS INDEX_M
FROM
    INFORMATION_SCHEMA.TABLES
WHERE
    TABLE_SCHEMA = 'godbart_prd_main'
ORDER BY 
    TABLE_ROWS DESC, 
    DATA_M DESC;
```

### 4.3. 调整分叉位置

多分支的`REF`会生成多个分叉的节点，可以通过`FOR`和`END`调整。

而依赖与多个条件的`WHERE`，可`JOIN`到同一个分叉SQL中。

以 `./demo/sql/tree/fork.sql` 为例。

### 4.4. REF的默认值

当REF的SQL返回的0条记录时，以此为根的树就不会存在。
我们可以通过以下的SQL来指定默认值，保证能返回1条记录。

```mysql
-- 通过 INSERT IGNORE 插入默认值
INSERT IGNORE SYS_HOT_SEPARATION VALUES ('tx_parcel',0, NOW());

-- 批量初始化
INSERT IGNORE SYS_HOT_SEPARATION SELECT 
    TABLE_NAME,0,NOW()
FROM
    INFORMATION_SCHEMA.TABLES
WHERE
    TABLE_SCHEMA = 'godbart_prd_main';

-- 通过 聚集函数与CASE WEN
SELECT 
    CASE
        WHEN MAX(CHECKED_ID) IS NULL THEN 0
        ELSE MAX(CHECKED_ID)
    END AS CHECKED_ID
FROM
    SYS_HOT_SEPARATION
WHERE
    TABLE_NAME = 'tx_parcel';
```

### 4.５. 全部同步

可以使用 `sync -t row` 进行小表的数据同步，也可以使用 `tree`的以下脚本。
这些脚本可以使用正则进行批量生成，参考`攻城狮朋友圈`正则分享。

```mysql
-- STR SRC-DB SRCDB
-- VAR checked_id 'tx_sender.checked_id'
select checked_id from sys_hot_separation where table_name = 'tx_sender';

-- REF max_id 'tx_sender.max_id'
select max(id) as max_id from tx_sender where id > 'tx_sender.checked_id';

-- OUT FOR 'tx_sender.max_id'
replace into tx_sender
  select * from SRCDB.tx_sender
  where id > 'tx_sender.checked_id' and id <= 'tx_sender.max_id';

-- RUN FOR 'tx_sender.max_id'
replace into sys_hot_separation values ('tx_sender', 'tx_sender.max_id', now());
```

### 4.6. 如何对比迁移数据
统计各表的数据变化，查看迁移效果
```mysql
-- 统计库数据
SELECT 
    TABLE_SCHEMA,
    SUM(TABLE_ROWS)
FROM
    INFORMATION_SCHEMA.TABLES
WHERE
    TABLE_SCHEMA like 'godbart_%'
GROUP BY 
    TABLE_SCHEMA;

-- 统计表数据
SELECT 
    TABLE_NAME,
    TABLE_ROWS
FROM
    INFORMATION_SCHEMA.TABLES
WHERE
    TABLE_SCHEMA = 'godbart_prd_main'
    AND TABLE_ROWS > 0
ORDER BY 
    TABLE_ROWS DESC;
```

### 4.7. 如何静态分析和运行时监控

静态分析，第一步，要执行sqlx命令，分析树结构。第二部，不带agree参数在db上执行以下，看debug日志。
运行时监控，使用控制端口，telnet连接过去，使用`stat|wait|tree`命令，还有本机日志。

### 4.8. `tree`做版本管理（分表）

除了`revi`，使用`tree`的`VAR和RUN FOR`也可以完成版本更新的。

```mysql
-- 单表 =========================
-- VAR VER v2019010302
SELECT MAX(version) as VER FROM sys_schema_version WHERE version = 2019010302;
-- RUN NOT v2019010302
ALTER TABLE tx_parcel ADD CONSTRAINT uk_track_num UNIQUE (is_deleted, track_num);
-- RUN NOT v2019010302
REPLACE INTO sys_schema_version (version, created) VALUES(2019010302, NOW());

-- 分表 =========================
-- VAR VER v2019010302
SELECT MAX(version) as VER FROM sys_schema_version WHERE version = 2019010302;
-- RUN NOT v2019010302
-- STR tbl `tx_parcel_#` 为分表更新
SELECT tbl FROM (
  SELECT 'tx_parcel_0' AS tbl  UNION ALL
  SELECT 'tx_parcel_1' UNION ALL
  SELECT 'tx_parcel_2' UNION ALL
  SELECT 'tx_parcel_3') TMP;
-- RUN NOT v2019010302
ALTER TABLE `tx_parcel_#` ADD CONSTRAINT uk_track_num UNIQUE (is_deleted, track_num);
-- RUN NOT v2019010302
REPLACE INTO sys_schema_version (version, created) VALUES(2019010302, NOW());

-- 分表 =========================== 0.9.7+

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
```

## 5. 不想理你的问题

* Q01：使用中发现了问题，出现了BUG怎么办？
  - 有能力hack code的，就提交PR。
  - 没能力的，提交 issue。
  - 再不行的，就认命吧。

* Q02：我SQL写错了，习惯性输入了`--agree`，结果数据丢了 :(
  - 事后没有后悔药，不要轻易 agree。
  - 执行前要确认，要两人确认，想好fallback计划。
  - 一定写where false的条件安全SQL。
  - 甚至写替换前语法错误的SQL。

* Q03：`FOR`中只有`HAS`和`NOT`，会增加`>`,`<`或其他运算符么？
  - 复杂的条件判断，可以由SQL语句产生，然后`REF|VAR`。
  - 写那么复杂的SQL，不如去编程好了。
  
* Q04：数据树迁移的吞吐量/性能如何？
  - 坏消息是吞吐量不太好，好消息是不占资源。
  - 实测一棵4层100条SQL的数据树，同机同实例千万数据，每秒迁移10.87棵树。
  - 速度依赖于sql索引，golang层面提升不大。
  
* Q05：输出信息太多了/太快了，看不清/来不及处理
  - 使用`-l trace`调整信息输出级别。
  - 用 shell的重定向分离信息流。
  - 看文档，像吃药一样，看说明书，听医嘱。

* Q06：SQL没有正常解析，报错了。
  - 确认单个完整的SQL中间没有空行分开，结尾有分隔符。
  - 确认一组SQL间，每个独立SQL有分隔符或空行分开。
  - 发个issue，贴上SQL，应该时没见过的SQL。

