package art

import (
	"bufio"
	"bytes"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	CtrlExitcd      = 99
	CtrlRoomTree    = "tree"
	roomTreeEnvSqlx = "tree_env_cur_sqlx"
	roomTreeEnvStat = "tree_env_cur_stat"
	//
	passLength = 24
	passTables = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ" // 32

	//
	roomBaseExit = "exit"
	roomBaseInfo = "info"
	roomBasePass = "pass"
	roomBaseKill = "kill"
	roomBaseHelp = "help"
	//
	roomTreeSqlx = "tree"
	roomTreeStat = "stat"
	roomTreeStop = "stop"
	roomTreeWait = "wait"

	helpBase = `
help - show help message.
exit - close this session.
pass - replace the password.
info - show room's info (pid,user,jobs...)'.
kill N - kill id=N job. N=-1 means kill all.
`
	helpTreeTree = `
tree - show the running sqlx tree
`
	helpTreeStat = `
stat - show the running statistic
`
	helpTreeStop = `
stop - gracefully stop when tree done (exit 99)
 * stop - stop at current tree.
 * stop N - stop at the line-number=N tree.
`
	helpTreeWait = `
wait - waiting when tree done, kill to continue. (can cause DB timeout)
 * wait - wait at current tree.
 * wait N wait at the line-number=N tree.
`
)

type CtrlJob struct {
	id   int64  // 任务id
	cmnd string // 命令
	user string // 提交者
	time string // 提交时间
	solo bool   // 单独发送
}

func (j *CtrlJob) String() string {
	return fmt.Sprintf("{id=%d, cmnd=%q, user=%s, time=%s}", j.id, j.cmnd, j.user, j.time)
}

type Room struct {
	pid  int         // 进程ID
	port int         // 服务端口
	name string      // 房间名
	pass string      // 房间密码
	boff bool        // 房间开放
	help []byte      // 帮助信息
	cmdw []string    // 等待执行的命令
	cmdi []string    // 立即执行的命令
	echo chan string // 回现的信息
	user sync.Map    // 连接的用户

	envs sync.Map // 任务需要的参数
	jobs sync.Map // 当前的命令
	jbid int64    // 已分批的 job id
	jcid int64    // 待执行的 job id
}

func (room *Room) Open(port int, name string, wait *sync.WaitGroup) {
	if port <= 0 {
		LogTrace("skip ControlPort, name=%s, port=%d", name, port)
	}

	// 创建房间
	room.pid = os.Getpid()
	room.port = port
	room.name = name
	room.pass = makePass()
	room.jcid = 0
	room.jbid = 0
	room.boff = true

	switch name {
	case CtrlRoomTree:
		room.help = []byte(helpBase + helpTreeTree + helpTreeStat + helpTreeStop + helpTreeWait)
		room.cmdw = []string{roomTreeStop, roomTreeWait}
		room.cmdi = []string{roomTreeSqlx, roomTreeStat}
		room.echo = make(chan string)
		room.boff = false
	default:
		LogFatal("unsupported room %s", name)
	}

	// 监听端口，单例控制
	ntw := fmt.Sprintf("0.0.0.0:%d", port)
	server, err := net.Listen("tcp", ntw)
	if err != nil {
		es := err.Error()
		if strings.Contains(es, "address already in use") {
			info := askInfo(ntw)
			es = fmt.Sprintf("an instant is running. %s", info)
		}
		LogFatal("%s", es)
	}

	LogTrace("CONTROLPORT started, port=%d, pid=%d, PASS=%s", port, room.pid, room.pass)

	//
	if wait != nil {
		wait.Done()
	}

	defer server.Close()

	go room.gogoTalk()
	for {
		conn, err := server.Accept()
		if err != nil {
			LogError("skip a bad client. error=%v", err)
			continue
		}
		go room.gogoConn(conn)
	}
}

var (
	bytesAuth = []byte("need password to auth\r\n")
	bytesUnsp = []byte("unsupported control command\r\n")
)

func (room *Room) infoByte(user string) []byte {
	var sb bytes.Buffer
	sb.WriteString(fmt.Sprintf("\r\npid  = %d", room.pid))
	sb.WriteString(fmt.Sprintf("\r\nroom = %s", room.name))
	room.jobs.Range(func(k, v interface{}) bool {
		jb := v.(*CtrlJob)
		sb.WriteString(fmt.Sprintf("\r\njob=%d, user=%s, cmnd=%s", jb.id, jb.user, jb.cmnd))
		return true
	})
	room.user.Range(func(k, v interface{}) bool {
		u, m := k.(string), ""
		if u == user {
			m = "*"
		}
		sb.WriteString(fmt.Sprintf("\r\nuser = %s %s", u, m))
		return true
	})
	return sb.Bytes()
}

func (room *Room) putJob(cmnd, user string, solo bool) {
	id := atomic.AddInt64(&room.jbid, 1)
	dt := time.Now().Format("15:04:05")
	jb := &CtrlJob{id, cmnd, user, dt, solo}
	room.jobs.Store(id, jb)
	text := fmt.Sprintf("job applied, job=%d, user=%s, cmnd=%s", jb.id, jb.user, jb.cmnd)
	room.echo <- text
	LogTrace(text)
}

func (room *Room) delJob(user string, id int64) {
	if id < 0 {
		room.jobs.Range(func(k, v interface{}) bool {
			room.jobs.Delete(k)
			return true
		})
		LogTrace("killed all jobs, user=%s", user)
		room.echo <- fmt.Sprintf("killed all jobs, user=%s", user)
	} else {
		room.jobs.Delete(id);
		LogTrace("job id=%d killed by user=%s", id, user)
		room.echo <- fmt.Sprintf("job id=%d killed by user=%s", id, user)
	}
}

func (room *Room) gogoConn(conn net.Conn) {
	user := conn.RemoteAddr().String()
	authed := strings.HasPrefix(user, "127.0.0.")

	if authed {
		conn.Write(room.infoByte(user))
		conn.Write(makeProm())
	} else {
		time.Sleep(time.Second * 5)
	}

	defer func() {
		LogTrace("client %s is closed.", user)
		room.dealEcho(user, "user logout, "+user, false)
		room.user.Delete(user)
		conn.Close()
	}()

	reader := bufio.NewReader(conn)
	// auth
	for !authed {
		conn.Write(bytesAuth)
		pass, er := reader.ReadString('\n')
		if er != nil && strings.TrimSpace(pass) == room.pass {
			conn.Write(room.infoByte(user))
			conn.Write(makeProm())
			authed = true
			break
		} else {
			return // one time
		}
	}

	room.user.Store(user, conn)
	room.dealEcho(user, "user loging, "+user, false)

	// command
	for ln, err := reader.ReadString('\n'); err == nil; ln, err = reader.ReadString('\n') {
		ln = strings.Replace(ln, "\t", " ", -1);
		ln = strings.TrimSpace(ln);
		switch part := strings.SplitN(ln, " ", 2); part[0] {
		case "":
			continue
		case roomBaseExit:
			return
		case roomBaseHelp:
			conn.Write(room.help)
		case roomBaseInfo:
			conn.Write(room.infoByte(user))
		case roomBaseKill:
			var jbid int64
			if len(part) > 1 {
				id, er := strconv.ParseInt(part[1], 10, 64)
				if er != nil {
					conn.Write([]byte(fmt.Sprintf("bad job id %s, err=%s", ln, er.Error())));
					continue
				}
				jbid = int64(id)
			} else {
				jbid = -1
			}
			room.delJob(user, jbid)
		case roomBasePass:
			room.pass = makePass()
			conn.Write([]byte(fmt.Sprintf("NEW-PASS=%s\r\n", room.pass)))
			LogTrace("client %s chagned pass. NEW-PASS=%s", user, room.pass)
			room.echo <- user + " changed room pass."
		default:
			if strings.HasPrefix(ln, "/") {
				room.echo <- fmt.Sprintf("%s <%s", ln, user)
				continue
			}

			fd := -1
			for _, v := range room.cmdi {
				if strings.HasPrefix(ln, v) {
					fd = 1
					break
				}
			}
			for _, v := range room.cmdw {
				if strings.HasPrefix(ln, v) {
					fd = 2
					break
				}
			}

			if fd == 1 {
				job := &CtrlJob{-1, ln, user, time.Now().Format("15:04:05"), true}
				room.dealJobx(job)
			} else if fd == 2 {
				room.putJob(ln, user, false)
			} else {
				conn.Write(bytesUnsp)
			}
		}
		conn.Write(makeProm())
	}
}

func (room *Room) gogoTalk() {
	for {
		info := <-room.echo // waiting
		if len(info) == 0 {
			continue
		} else if info == "CLOSE_ECHO" {
			room.boff = true
			close(room.echo)
			break
		}

		bytesProm := makeProm()
		if strings.HasPrefix(info, "/") {
			info = strings.TrimSpace(info[1:])
			var s, r net.Conn
			l := 0
			room.user.Range(func(k, v interface{}) bool {
				user := k.(string)
				if strings.HasPrefix(info, user) {
					l = len(user)
					s = v.(net.Conn)
				}
				if strings.HasSuffix(info, user) {
					r = v.(net.Conn)
				}
				return true
			})

			if s != nil {
				msg := []byte(strings.TrimSpace(info[l:] + "*"))
				s.Write(msg)
				s.Write(bytesProm)
				r.Write(msg)
				r.Write(bytesProm)
				continue
			}
		}

		msgs := []byte(info)
		room.user.Range(func(k, v interface{}) bool {
			conn := v.(net.Conn)
			conn.Write(msgs)
			conn.Write(bytesProm)
			return true
		})
	}
}

func (room *Room) dealEcho(user, text string, solo bool) {
	if solo {
		if con, ho := room.user.Load(user); ho {
			conn := con.(net.Conn)
			conn.Write([]byte(text))
			return
		}
	}
	room.echo <- text
}

// single thread
func (room *Room) dealJobx(job *CtrlJob, args ... interface{}) {
	if room.boff { // not init
		return
	}

	if job == nil {
		jbid := atomic.LoadInt64(&room.jbid)
		for i := atomic.LoadInt64(&room.jcid); i <= jbid; i = atomic.AddInt64(&room.jcid, 1) {
			jb, ok := room.jobs.Load(i)
			if ok {
				job = jb.(*CtrlJob)
				break
			}
		}
		if job == nil {
			return
		}
	}

	done := true
	defer func() {
		if done {
			room.jobs.Delete(job.id)
		}
	}()

	switch room.name {
	case CtrlRoomTree:

		if strings.HasPrefix(job.cmnd, roomTreeSqlx) {
			v1, ok := room.envs.Load(roomTreeEnvSqlx)
			if !ok {
				room.dealEcho(job.user, "can not find current sqlx", false)
				return
			}

			sqlx := v1.(*SqlExe)
			var sb strings.Builder
			for _, x := range sqlx.Exes {
				sb.WriteString(x.Tree())
			}

			room.dealEcho(job.user, sb.String(), job.solo)
			return
		}

		if strings.HasPrefix(job.cmnd, roomTreeStat) {
			v1, ok := room.envs.Load(roomTreeEnvStat)
			if !ok {
				room.dealEcho(job.user, "can not find current stat", false)
				return
			}
			para := v1.(*exeStat)
			scnd := time.Now().Sub(para.startd).Seconds()
			text := fmt.Sprintf("elapsed=%.2fs, tree/s=%.2f, src/s=%.2f, dst/s=%.2fs\r\ntrees=%d, select-row=%d, child-exe=%d\r\nsrc-affect=%d, dst-affect=%d",
				scnd, float64(para.cnttop)/scnd, float64(para.cntsrc)/scnd, float64(para.cntdst)/scnd,
				para.cnttop, para.cntrow, para.cntson,
				para.cntsrc, para.cntdst)

			room.dealEcho(job.user, text, job.solo)

			return
		}

		headRun, headArg := -1, -1
		if len(args) > 0 {
			headRun = args[0].(int)
		}

		part := strings.SplitN(job.cmnd, " ", 2)
		if len(part) > 1 {
			hd, er := strconv.ParseInt(part[1], 10, 32)
			if er != nil {
				return
			}
			headArg = int(hd)
		}

		LogTrace("tree at=%3d, job=%v", headRun, job)
		room.dealEcho(job.user, fmt.Sprintf("tree at=%3d, job=%d, user=%s, cmnd=%s\n", headRun, job.id, job.user, job.cmnd), false)

		if strings.HasSuffix(part[0], roomTreeStop) {
			if headArg < 0 {
				LogTrace("exited by %s", job.cmnd)
				room.dealEcho(job.user, fmt.Sprintf("exited in 5 seconds, by %s\n", job.cmnd), false)
				time.Sleep(time.Second * 5)
				os.Exit(CtrlExitcd)
			} else {
				if headRun == headArg {
					LogTrace("exited by %s", job.cmnd)
					room.dealEcho(job.user, fmt.Sprintf("exited in 5 seconds, by %s\n", job.cmnd), false)
					time.Sleep(time.Second * 5)
					os.Exit(CtrlExitcd)
				} else {
					done = false
				}
			}
		} else if strings.HasSuffix(part[0], roomTreeWait) {
			if headArg < 0 {
				LogTrace("waiting by %s", job.cmnd)
				room.dealEcho(job.user, fmt.Sprintf("waiting by %s", job.cmnd), false)
				for {
					time.Sleep(time.Second * 3)
					_, oh := room.jobs.Load(job.id)
					if !oh {
						room.dealEcho(job.user, fmt.Sprintf("resume from %s", job.cmnd), false)
						return
					}
				}
			} else {
				if headRun == headArg {
					LogTrace("waiting by %s", job.cmnd)
					room.dealEcho(job.user, fmt.Sprintf("waiting by %s", job.cmnd), false)
					for {
						time.Sleep(time.Second * 3)
						_, oh := room.jobs.Load(job.id)
						if !oh {
							room.dealEcho(job.user, fmt.Sprintf("resume from %s", job.cmnd), false)
							return
						}
					}
				} else {
					done = false
				}
			}
		}
	}
}

func (room *Room) putEnv(key string, val interface{}) {
	LogTrace("put room env key=%s", key)
	room.envs.Store(key, val)
}

func makePass() string {
	var sb strings.Builder
	tbl := len(passTables)
	for i := 0; i < passLength; i++ {
		j := rand.Intn(tbl)
		sb.WriteByte(passTables[j])
	}
	return sb.String()
}

func askInfo(ntw string) string {
	conn, err := net.Dial("tcp", ntw)
	if err != nil {
		return err.Error()
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(time.Second))
	reader := bufio.NewReader(conn)

	var sb strings.Builder
	for {
		line, er := reader.ReadString('\n')
		if er != nil {
			break
		}

		line = strings.TrimSpace(line);
		if len(line) == 0 {
			continue
		}
		if strings.Contains(line, "room") {
			sb.WriteString(line)
			break
		}
		sb.WriteString(line)
		sb.WriteString(", ")
	}
	return sb.String()
}

func makeProm() []byte {
	return []byte(fmt.Sprintf("\r\n%s >", time.Now().Format("15:04:05")))
}
