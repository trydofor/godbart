#!/bin/bash

out_dir="/tmp"

### #########

if [[ ! -x godbart ]]; then
    echo "### 重新编译 godbart"
    go build
fi

cat << EOF
## 进行演示之前，一定要设置好mysql的链接（用户，密码）。
## 会用到兼容linux的以下命令: tee, diff, grep。
## 留意控制台输出，没问题的话，按ENTER继续。
EOF

function wait_txt(){
    echo "### $1"
    read -p "### press ENTER to continue, Ctrl-C to break."
}

function echo_txt(){
    echo "### $1"
}

function diff_txt(){
    rm -rf $out_dir/01.out
    if [[ -z `diff -wBZ $1 $2` ]]; then
        echo "### $3 结果正确"
    else
        echo "!!! $3 结果错误"
        diff -wBZ $1 $2
        exit
    fi
}

wait_txt "初始化数据库"
./godbart exec \
 -c godbart.toml \
 -d lcl_test \
 --agree \
 demo/sql/diff/reset.sql
echo_txt "查询数据库"
./godbart tree \
 -c godbart.toml \
 -s lcl_test \
 demo/chk/sql/01.sql \
 | tee $out_dir/01.txt
diff_txt demo/chk/txt/01.txt $out_dir/01.txt "数据库创建:prd_main"


wait_txt "初始化数据:prd_main"
./godbart exec \
 -c godbart.toml \
 -d prd_main \
 --agree \
 demo/sql/init/
echo_txt "查询表结构"
./godbart diff \
 -c godbart.toml \
 -s prd_main \
 -t ddl \
 | tee $out_dir/02.txt
diff_txt demo/chk/txt/02.txt $out_dir/02.txt "表结构:prd_main"

echo_txt "查询表记录"
./godbart tree \
 -c godbart.toml \
 -s prd_main \
 demo/chk/sql/03.sql \
 | tee $out_dir/03.txt
diff_txt demo/chk/txt/03.txt $out_dir/03.txt "表记录:prd_main"


wait_txt "执行版本控制:prd_main,prd_2018"
./godbart revi \
 -c godbart.toml \
 -d prd_main \
 -d prd_2018 \
 -r 2018111103 \
 --agree \
 demo/sql/revi/
echo_txt "查询版本表结构:prd_2018"
./godbart diff \
 -c godbart.toml \
 -s prd_2018 \
 -t ddl \
 | tee $out_dir/04.txt
diff_txt demo/chk/txt/04.txt $out_dir/04.txt "数据库表结构:prd_2018"

wait_txt "同步表结构:prd_main,dev_main"
./godbart sync \
 -c godbart.toml \
 -s prd_main \
 -d dev_main \
 -t all \
 --agree
echo_txt "同步版本号:dev_main"
./godbart sync \
 -c godbart.toml \
 -s prd_main \
 -d dev_main \
 -t row \
 --agree \
 sys_schema_version
echo_txt "查询表结构:dev_main"
./godbart diff \
 -c godbart.toml \
 -s dev_main \
 -t ddl \
 | tee $out_dir/05.txt
diff_txt demo/chk/txt/05.txt $out_dir/05.txt "数据库表结构:dev_main"

wait_txt "静态分析 sqlx-tree.log"
./godbart sqlx \
 -c godbart.toml \
 -e "DATE_FROM=2018-01-01 00:00:00" \
 -l trace \
 demo/sql/tree/tree.sql \
 | tee $out_dir/06.txt
diff_txt demo/chk/txt/06.txt $out_dir/06.txt "数据树结构:tree.sql"


wait_txt "迁移数据 prd_main:prd_2018"
./godbart tree \
 -c godbart.toml \
 -s prd_main \
 -d prd_2018 \
 -e "DATE_FROM=2018-01-01 00:00:00" \
 --agree \
 demo/sql/tree/tree.sql \
| tee  $out_dir/07.txt
diff_txt demo/chk/txt/07.txt $out_dir/07.txt "迁移数据过程:tree.sql"

wait_txt "对比迁移数据结果:prd_main"
./godbart tree \
 -c godbart.toml \
 -s prd_main \
 demo/chk/sql/03.sql \
 | tee $out_dir/08.txt
diff_txt demo/chk/txt/08.txt $out_dir/08.txt "对比迁移数据结果:prd_main"


echo_txt "对比迁移数据结果:prd_2018"
./godbart tree \
 -c godbart.toml \
 -s prd_2018 \
 demo/chk/sql/03.sql \
 | tee $out_dir/09.txt
diff_txt demo/chk/txt/09.txt $out_dir/09.txt "对比迁移数据结果:prd_2018"

wait_txt "高级版本管理:dev_main"
./godbart revi \
 -c godbart.toml \
 -d dev_main \
 -r 2019011101 \
 --agree \
 demo/sql/revi/2019-01-11.sql

echo_txt "查询表结构:dev_main"
./godbart diff \
 -c godbart.toml \
 -s prd_main \
 -d dev_main \
 -t all \
 | tee $out_dir/10.txt
diff_txt demo/chk/txt/10.txt $out_dir/10.txt "数据库表结构:dev_main"

