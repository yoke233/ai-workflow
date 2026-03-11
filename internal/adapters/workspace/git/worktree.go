package git

import "os/exec"

func (r *Runner) WorktreeAdd(path, branch string) error {
	_, err := r.run("worktree", "add", "-b", branch, path)
	return err
}

func (r *Runner) WorktreeRemove(path string) error {
	_, err := r.run("worktree", "remove", path, "--force")
	return err
}

func (r *Runner) WorktreeClean(path string) error {
	cmd1 := exec.Command("git", "-C", path, "checkout", ".")
	if err := cmd1.Run(); err != nil {
		return err
	}
	cmd2 := exec.Command("git", "-C", path, "clean", "-fd")
	return cmd2.Run()
}
