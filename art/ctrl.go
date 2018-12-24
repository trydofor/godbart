package art

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
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
tree - show the running sqlx
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

var CtrlRoom = &Room{}

func (room *Room) Open(port int, name string) {
	if port <= 0 {
		log.Printf("[TRACE] skip ControlPort, name=%s, port=%d", name, port)
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
		room.help = []byte(helpBase + helpTreeTree + helpTreeStop + helpTreeWait)
		room.cmdw = []string{roomTreeStop, roomTreeWait}
		room.cmdi = []string{roomTreeSqlx}
		room.echo = make(chan string)
		room.boff = false
	default:
		log.Fatalf("[ERROR] unsupported room %s\n", name)
		os.Exit(CtrlExitcd)
	}

	// 监听端口，单例控制
	ntw := fmt.Sprintf("0.0.0.0:%d", port)
	server, err := net.Listen("tcp", ntw)
	if err != nil {
		es := err.Error()
		if strings.Contains(es, "address already in use") {
			info := askInfo(ntw)
			es = fmt.Sprintf("an instant is runing info=%s", info)
		}
		log.Fatalf("[ERROR] %s\n", es)
		os.Exit(CtrlExitcd)
	}

	log.Printf("[TRACE] CONTROLPORT started, port=%d, pid=%d, PASS=%s\n", port, room.pid, room.pass)

	//
	defer server.Close()

	go room.dealTalk()
	for {
		conn, err := server.Accept()
		if err != nil {
			log.Fatalf("[ERROR] a bad client connection error=%v\n", err)
		}
		go room.dealConn(conn)
	}
}

var (
	bytesProm = []byte("\r\n>")
	bytesAuth = []byte("need password to auth\r\n")
	bytesUnsp = []byte("unsupported control command\r\n")
)

func (room *Room) infoByte(user string) []byte {
	var sb bytes.Buffer
	sb.WriteString(fmt.Sprintf("\r\npid  = %d", room.pid))
	sb.WriteString(fmt.Sprintf("\r\nroom = %s", room.name))
	room.jobs.Range(func(k, v interface{}) bool {
		sb.WriteString(fmt.Sprintf("\r\n%#v", v))
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

func (room *Room) putJob(cmnd, user string) {
	id := atomic.AddInt64(&room.jbid, 1)
	dt := time.Now().Format("2006-01-02 15:04:05")
	jb := &CtrlJob{id, cmnd, user, dt}
	room.jobs.Store(id, jb)
	room.echo <- fmt.Sprintf("job=%#v applied", jb)
	log.Printf("[TRACE] job=%#v applied", jb)
}

func (room *Room) delJob(user string, id int64) {
	if id < 0 {
		room.jobs.Range(func(k, v interface{}) bool {
			room.jobs.Delete(k)
			return true
		})
		log.Printf("[TRACE] killed all jobs, user=%s", user)
		room.echo <- fmt.Sprintf("killed all jobs, user=%s", user)
	} else {
		room.jobs.Delete(id);
		log.Printf("[TRACE] job id=%d killed by user=%s", id, user)
		room.echo <- fmt.Sprintf("job id=%d killed by user=%s", id, user)
	}
}

func (room *Room) dealConn(conn net.Conn) {
	user := conn.RemoteAddr().String()
	defer func() {
		log.Printf("[TRACE] client %s is closed.\n", user)
		room.user.Delete(user)
		conn.Close()
	}()

	reader := bufio.NewReader(conn)
	authed := strings.HasPrefix(user, "127.0.0.")

	// auth
	for !authed {
		conn.Write(bytesAuth)
		pass, _ := reader.ReadString('\n')
		if strings.TrimSpace(pass) == room.pass {
			authed = true
			break
		} else {
			// one time
			return
		}
	}

	// command
	room.user.Store(user, conn)
	conn.Write(bytesProm)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.Replace(line, "\t", " ", -1);
		line = strings.TrimSpace(line);
		switch part := strings.SplitN(line, " ", 2); part[0] {
		case "":
			continue
		case roomBaseExit:
			return
		case roomBaseHelp:
			conn.Write(room.help)
		case roomBaseInfo:
			conn.Write(room.infoByte(user))
		case roomBaseKill:
			jbid, er := strconv.ParseInt(part[1], 10, 32)
			if er != nil {
				conn.Write([]byte(fmt.Sprintf("bad job id %s, err=%s", line, er.Error())));
				continue
			}
			room.delJob(user, jbid)
		case roomBasePass:
			room.pass = makePass()
			conn.Write([]byte(fmt.Sprintf("NEW-PASS=%s\r\n", room.pass)))
			log.Printf("[TRACE] client %s chagned pass. NEW-PASS=%s\n", user, room.pass)
			room.echo <- user + " changed room pass."
		default:
			if strings.HasPrefix(line, "/") {
				room.echo <- fmt.Sprintf("%s <%s", line, user)
				continue
			}

			fd := -1
			for _, v := range room.cmdi {
				if strings.HasPrefix(line, v) {
					fd = 1
					break
				}
			}
			for _, v := range room.cmdw {
				if strings.HasPrefix(line, v) {
					fd = 2
					break
				}
			}

			if fd == 1 {
				job := &CtrlJob{-1, line, user, time.Now().Format("2006-01-02 15:04:05")}
				room.dealJobx(job)
			} else if fd == 2 {
				room.putJob(line, user)
			} else {
				conn.Write(bytesUnsp)
			}
		}
		conn.Write(bytesProm)
	}
}

func (room *Room) dealTalk() {
	for {
		info := <-room.echo // waiting
		if len(info) == 0 {
			continue
		} else if info == "CLOSE_ECHO" {
			room.boff = true
			close(room.echo)
			break
		}

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
		v1, ok := room.envs.Load(roomTreeEnvSqlx)
		if !ok {
			room.echo <- "can not find current sqlx"
			return
		}

		sqlx := v1.(*SqlExe)
		if strings.HasSuffix(job.cmnd, roomTreeSqlx) {
			var sb strings.Builder
			for _, x := range sqlx.Exes {
				sb.WriteString(x.Tree())
			}
			room.echo <- sb.String()
			return
		}

		headRun, headArg := -1, -1
		part := strings.SplitN(job.cmnd, " ", 2)
		if len(part) > 1 {
			hd, er := strconv.ParseInt(part[1], 10, 32)
			if er != nil {
				return
			}
			headArg = int(hd)

			if len(args) < 1 {
				return
			}
			headRun = args[0].(int)
		}

		log.Printf("[TRACE] current id=%d, job=%#v\n", headRun, job)
		room.echo <- fmt.Sprintf("current id=%d, job=%#v\n", headRun, job)

		if strings.HasSuffix(part[0], roomTreeStop) {
			if headArg < 0 {
				log.Printf("[TRACE] exited by %s\n", job.cmnd)
				room.echo <- fmt.Sprintf("exited by %s\n", job.cmnd)
				time.Sleep(time.Second * 5)
				os.Exit(CtrlExitcd)
			} else {
				if headRun == headArg {
					log.Printf("[TRACE] exited by %s\n", job.cmnd)
					room.echo <- fmt.Sprintf("exited by %s\n", job.cmnd)
					time.Sleep(time.Second * 5)
					os.Exit(CtrlExitcd)
				} else {
					done = false
				}
			}
		} else if strings.HasSuffix(part[0], roomTreeWait) {
			if headArg < 0 {
				log.Printf("[TRACE] waiting by %s\n", job.cmnd)
				for {
					time.Sleep(time.Second * 3)
					_, oh := room.jobs.Load(job.id)
					if !oh {
						return
					}
				}
			} else {
				if headRun == headArg {
					log.Printf("[TRACE] waiting by %s\n", job.cmnd)
					for {
						time.Sleep(time.Second * 3)
						_, oh := room.jobs.Load(job.id)
						if !oh {
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
	log.Printf("[TRACE] put room env key=%s, val=%#v\n", key, val)
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

	conn.Write([]byte(roomBaseInfo))
	reader := bufio.NewReader(conn)
	line, _, _ := reader.ReadLine()
	return string(line)
}
