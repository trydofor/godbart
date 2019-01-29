#!/bin/bash

out_dir="/tmp/godbart" # 工作目录
gui_dff="meld" # 比较文件的gui工具

### #########

if [[ ! -x godbart ]]; then
    echo "### 重新编译 godbart"
    go build
fi

echo -e "\e[0;34m
## 进行演示之前，一定要设置好mysql的链接（用户，密码）。
## 会用到兼容linux的以下命令: tee, diff, grep
## 留意控制台输出，没问题的话，按ENTER继续。\e[m
"
rm   -rf "$out_dir"
mkdir -p "$out_dir"

function wait_txt(){
    echo -e "\e[0;34m### $1\e[m"
    read -p "### press ENTER to continue, Ctrl-C to break."
}

function echo_txt(){
    echo -e "\e[0;34m### $1\e[m"
}

function diff_txt(){

    if [[ ! -f $1 || ! -f $2 ]]; then
        echo -e "\e[0;31m### $3 结果文件不存在\e[m"
        exit
    fi

    if [[ ! -z `grep -E 'ERROR|FATAL' $2` ]]; then
        echo -e "\e[0;31m### $3 执行日志中有错误\e[m"
        exit
    fi

    # 分离结果
    out=$2.out
    grep -vE '^[0-9]{4}[^0-9][0-9]{2}' $2 >$out

    if [[ -z `diff -wBZ $1 $out` ]]; then
        echo -e "\e[0;32m### $3 结果正确\e[m"
        rm -rf "$out"
    else
        echo -e "\e[0;31m!!! $3 结果对比不一致\e[m"
        echo "diff -wBZ $1 $out"
        $gui_dff $1 $out
        exit
    fi
}

wait_txt "初始化数据库"
./godbart exec \
 -c godbart.toml \
 -d lcl_test \
 --agree \
 demo/sql/diff/reset.sql \
 2>&1| tee $out_dir/01.txt \
 |grep -E '^[0-9]{4}[^0-9][0-9]{2}'
wait_txt "查询数据库"
./godbart tree \
 -c godbart.toml \
 -s lcl_test \
 demo/chk/sql/01.sql \
 2>&1| tee -a $out_dir/01.txt \
 |grep -E '^[0-9]{4}[^0-9][0-9]{2}'
diff_txt demo/chk/txt/01.txt $out_dir/01.txt "数据库创建:prd_main"


wait_txt "初始化数据:prd_main"
./godbart exec \
 -c godbart.toml \
 -d prd_main \
 --agree \
 demo/sql/init/ \
 2>&1| tee $out_dir/02.txt \
 |grep -E '^[0-9]{4}[^0-9][0-9]{2}'
wait_txt "查询表结构"
./godbart show \
 -c godbart.toml \
 -s prd_main \
 -t tbl,trg \
 2>&1| tee -a $out_dir/02.txt \
 |grep -E '^[0-9]{4}[^0-9][0-9]{2}'
diff_txt demo/chk/txt/02.txt $out_dir/02.txt "表结构:prd_main"

wait_txt "查询表记录"
./godbart tree \
 -c godbart.toml \
 -s prd_main \
 demo/chk/sql/03.sql \
 2>&1| tee $out_dir/03.txt \
 |grep -E '^[0-9]{4}[^0-9][0-9]{2}'
diff_txt demo/chk/txt/03.txt $out_dir/03.txt "表记录:prd_main"


wait_txt "执行版本控制:prd_main,prd_2018"
./godbart revi \
 -c godbart.toml \
 -d prd_main \
 -d prd_2018 \
 -r 2018111103 \
 --agree \
 demo/sql/revi/ \
 2>&1| tee $out_dir/04.txt \
 |grep -E '^[0-9]{4}[^0-9][0-9]{2}'
wait_txt "查询版本表结构:prd_2018"
./godbart show \
 -c godbart.toml \
 -s prd_2018 \
 -t tbl,trg \
 2>&1| tee -a $out_dir/04.txt \
 |grep -E '^[0-9]{4}[^0-9][0-9]{2}'
diff_txt demo/chk/txt/04.txt $out_dir/04.txt "数据库表结构:prd_2018"

wait_txt "同步表结构:prd_main,dev_main"
./godbart sync \
 -c godbart.toml \
 -s prd_main \
 -d dev_main \
 -t tbl,trg \
 --agree
wait_txt "同步版本号:dev_main"
./godbart sync \
 -c godbart.toml \
 -s prd_main \
 -d dev_main \
 -t row \
 --agree \
 sys_schema_version
wait_txt "查询表结构:dev_main"
./godbart show \
 -c godbart.toml \
 -s dev_main \
 -t tbl,trg \
 2>&1| tee $out_dir/05.txt \
 |grep -E '^[0-9]{4}[^0-9][0-9]{2}'
diff_txt demo/chk/txt/05.txt $out_dir/05.txt "数据库表结构:dev_main"

wait_txt "静态分析 sqlx-tree.log"
./godbart sqlx \
 -c godbart.toml \
 -e "DATE_FROM=2018-01-01 00:00:00" \
 -l trace \
 demo/sql/tree/tree.sql \
 2>&1| tee $out_dir/06.txt
diff_txt demo/chk/txt/06.txt $out_dir/06.txt "数据树结构:tree.sql"


wait_txt "迁移数据 prd_main:prd_2018"
./godbart tree \
 -c godbart.toml \
 -s prd_main \
 -d prd_2018 \
 -e "DATE_FROM=2018-01-01 00:00:00" \
 --agree \
 demo/sql/tree/tree.sql \
 2>&1| tee  $out_dir/07.txt \
 |grep -E '^[0-9]{4}[^0-9][0-9]{2}'
diff_txt demo/chk/txt/07.txt $out_dir/07.txt "迁移数据过程:tree.sql"

wait_txt "对比迁移数据结果:prd_main"
./godbart tree \
 -c godbart.toml \
 -s prd_main \
 demo/chk/sql/03.sql \
 2>&1| tee $out_dir/08.txt \
 |grep -E '^[0-9]{4}[^0-9][0-9]{2}'
diff_txt demo/chk/txt/08.txt $out_dir/08.txt "对比迁移数据结果:prd_main"


wait_txt "对比迁移数据结果:prd_2018"
./godbart tree \
 -c godbart.toml \
 -s prd_2018 \
 demo/chk/sql/03.sql \
 2>&1| tee $out_dir/09.txt \
 |grep -E '^[0-9]{4}[^0-9][0-9]{2}'
diff_txt demo/chk/txt/09.txt $out_dir/09.txt "对比迁移数据结果:prd_2018"

wait_txt "高级版本管理:dev_main"
./godbart revi \
 -c godbart.toml \
 -d dev_main \
 -r 2019011101 \
 --agree \
 demo/sql/revi/2019-01-11.sql \
 2>&1| tee $out_dir/10.txt \
 |grep -E '^[0-9]{4}[^0-9][0-9]{2}'
wait_txt "查询表结构:dev_main"
./godbart diff \
 -c godbart.toml \
 -s prd_main \
 -d dev_main \
 -t tbl,trg \
 2>&1| tee -a $out_dir/10.txt \
 |grep -E '^[0-9]{4}[^0-9][0-9]{2}'
diff_txt demo/chk/txt/10.txt $out_dir/10.txt "数据库表结构:dev_main"

echo_txt "====所有测试结束==="