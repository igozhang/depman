package deploy

import (
	"context"
	"log"
	"time"

	connssh "depman/internal/connector/ssh"
	"depman/internal/model"
	"depman/internal/store"
)

type Runner struct {
	Store *store.Store
}

func (r *Runner) Start(jobID string) {
	go r.run(jobID)
}

func (r *Runner) run(jobID string) {
	job, err := r.Store.GetDeployJob(jobID)
	if err != nil {
		log.Printf("deploy: get job %s: %v", jobID, err)
		return
	}
	pe, err := r.Store.GetProjectEnv(job.ProjectEnvID)
	if err != nil {
		_ = r.fail(jobID, "", err.Error())
		return
	}
	if pe.SSHCredID == "" {
		_ = r.fail(jobID, "", "未配置 SSH 凭据")
		return
	}
	cred, secret, err := r.Store.PlainSecret(pe.SSHCredID)
	if err != nil {
		_ = r.fail(jobID, "", "读取 SSH 凭据失败: "+err.Error())
		return
	}
	start := time.Now()
	_ = r.Store.UpdateDeployJob(jobID, "running", "", "", &start, nil)
	_ = r.Store.MarkProjectEnvDeploy(pe.ID, "running", start)

	cli := connssh.New(connssh.ConfigFromSecret(cred.Host, cred.Port, cred.Username, secret))
	out, err := cli.Run(context.Background(), pe.WorkDir, pe.Script, 15*time.Minute)
	fin := time.Now()
	if err != nil {
		_ = r.Store.UpdateDeployJob(jobID, "failed", out, err.Error(), &start, &fin)
		_ = r.Store.MarkProjectEnvDeploy(pe.ID, "failed", fin)
		return
	}
	_ = r.Store.UpdateDeployJob(jobID, "ok", out, "", &start, &fin)
	_ = r.Store.MarkProjectEnvDeploy(pe.ID, "ok", fin)
}

func (r *Runner) fail(jobID, logTail, msg string) error {
	fin := time.Now()
	return r.Store.UpdateDeployJob(jobID, "failed", logTail, msg, nil, &fin)
}

func (r *Runner) Enqueue(pe model.ProjectEnv, createdBy string) (model.DeployJob, error) {
	job, err := r.Store.CreateDeployJob(pe.ProjectID, pe.EnvID, pe.ID, pe.Script, createdBy)
	if err != nil {
		return job, err
	}
	r.Start(job.ID)
	return job, nil
}
