package git

import "strings"

// DetectDefaultBranch returns the current branch of repoPath, falling back to "main".
func DetectDefaultBranch(repoPath string) string {
	r := NewRunner(repoPath)
	b, err := r.CurrentBranch()
	if err != nil {
		return "main"
	}
	return b
}

func (r *Runner) BranchDelete(name string) error {
	_, err := r.run("branch", "-D", name)
	return err
}

func (r *Runner) CurrentBranch() (string, error) {
	return r.run("rev-parse", "--abbrev-ref", "HEAD")
}

func (r *Runner) Merge(branch string) (string, error) {
	return r.run("merge", branch, "--no-ff", "-m", "Merge "+branch)
}

func (r *Runner) Checkout(branch string) error {
	_, err := r.run("checkout", branch)
	return err
}

func (r *Runner) HasUncommittedChanges() (bool, error) {
	out, err := r.run("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func (r *Runner) AddAll() error {
	_, err := r.run("add", "-A")
	return err
}

func (r *Runner) Commit(msg string) error {
	_, err := r.run("commit", "-m", msg)
	return err
}
