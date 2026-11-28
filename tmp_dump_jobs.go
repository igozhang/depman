package main
import (
  "database/sql"
  "fmt"
  _ "modernc.org/sqlite"
)
func main() {
  db, err := sql.Open("sqlite", `E:\gitee\depman\data\depman.db`)
  if err != nil { panic(err) }
  defer db.Close()
  rows, err := db.Query(`SELECT id, status, script, log_tail, error, project_env_id FROM deploy_jobs ORDER BY created_at DESC LIMIT 8`)
  if err != nil { panic(err) }
  defer rows.Close()
  for rows.Next() {
    var id, status, script, log, errMsg, peid string
    _ = rows.Scan(&id, &status, &script, &log, &errMsg, &peid)
    fmt.Printf("--- job %s status=%s pe=%s\n", id, status, peid)
    fmt.Printf("script=%q\n", script)
    fmt.Printf("log=%q\n", log)
    fmt.Printf("err=%q\n", errMsg)
  }
  fmt.Println("=== project_envs ===")
  rows2, err := db.Query(`SELECT id, env_id, ssh_cred_id, script, work_dir, last_status FROM project_envs`)
  if err != nil { panic(err) }
  defer rows2.Close()
  for rows2.Next() {
    var id, env, cred, script, wd, st string
    _ = rows2.Scan(&id, &env, &cred, &script, &wd, &st)
    fmt.Printf("pe=%s env=%s cred=%s wd=%q status=%s\nscript=%q\n", id, env, cred, wd, st, script)
  }
  fmt.Println("=== credentials (no secret) ===")
  rows3, err := db.Query(`SELECT id, env_id, name, type, host, port, username FROM credentials`)
  if err != nil { panic(err) }
  defer rows3.Close()
  for rows3.Next() {
    var id, env, name, typ, host, user string
    var port int
    _ = rows3.Scan(&id, &env, &name, &typ, &host, &port, &user)
    fmt.Printf("cred=%s env=%s name=%s type=%s host=%s port=%d user=%s\n", id, env, name, typ, host, port, user)
  }
  fmt.Println("=== environments ===")
  rows4, err := db.Query(`SELECT id, name, kind FROM environments`)
  if err != nil { panic(err) }
  defer rows4.Close()
  for rows4.Next() {
    var id, name, kind string
    _ = rows4.Scan(&id, &name, &kind)
    fmt.Printf("env=%s name=%s kind=%s\n", id, name, kind)
  }
}
