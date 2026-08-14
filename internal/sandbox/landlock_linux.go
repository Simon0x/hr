//go:build linux

package sandbox

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// writeAccess is every filesystem right that changes something. Reads and
// execution are absent on purpose - see Policy.
const writeAccess = unix.LANDLOCK_ACCESS_FS_WRITE_FILE |
	unix.LANDLOCK_ACCESS_FS_MAKE_REG |
	unix.LANDLOCK_ACCESS_FS_MAKE_DIR |
	unix.LANDLOCK_ACCESS_FS_MAKE_SYM |
	unix.LANDLOCK_ACCESS_FS_MAKE_SOCK |
	unix.LANDLOCK_ACCESS_FS_MAKE_FIFO |
	unix.LANDLOCK_ACCESS_FS_MAKE_BLOCK |
	unix.LANDLOCK_ACCESS_FS_MAKE_CHAR |
	unix.LANDLOCK_ACCESS_FS_REMOVE_FILE |
	unix.LANDLOCK_ACCESS_FS_REMOVE_DIR |
	unix.LANDLOCK_ACCESS_FS_TRUNCATE

// Landlock confines through the kernel LSM of the same name. It needs no
// privileges and no setuid helper, which is why it is reachable here at all.
type Landlock struct{}

var _ Sandbox = Landlock{}

func (Landlock) Name() string { return "landlock" }

// Wrap re-enters hr as `hr confine`, because Landlock restricts the calling
// process and Go gives no hook between fork and exec. The helper restricts
// itself and then execs the real command, so the target starts already
// confined and cannot un-confine: a Landlock ruleset is irrevocable.
func (Landlock) Wrap(argv []string, p Policy) ([]string, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("nothing to confine")
	}
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	out := []string{self, "confine"}
	for _, w := range p.Writable {
		out = append(out, "--writable", w)
	}
	return append(append(out, "--"), argv...), nil
}

func available() bool {
	attr := unix.LandlockRulesetAttr{Access_fs: writeAccess}
	fd, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET,
		uintptr(unsafe.Pointer(&attr)), unsafe.Sizeof(attr), 0)
	if errno != 0 {
		return false
	}
	unix.Close(int(fd))
	return true
}

func apply(p Policy) error {
	attr := unix.LandlockRulesetAttr{Access_fs: writeAccess}
	fd, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET,
		uintptr(unsafe.Pointer(&attr)), unsafe.Sizeof(attr), 0)
	if errno != 0 {
		return fmt.Errorf("landlock_create_ruleset: %w", errno)
	}
	defer unix.Close(int(fd))

	for _, w := range p.Writable {
		dfd, err := unix.Open(w, unix.O_PATH|unix.O_CLOEXEC, 0)
		if err != nil {
			// A writable root that does not exist is not a reason to run
			// unconfined; skip it and keep the rest of the ruleset.
			continue
		}
		pb := unix.LandlockPathBeneathAttr{Allowed_access: writeAccess, Parent_fd: int32(dfd)}
		_, _, errno := unix.Syscall6(unix.SYS_LANDLOCK_ADD_RULE, fd,
			uintptr(unix.LANDLOCK_RULE_PATH_BENEATH), uintptr(unsafe.Pointer(&pb)), 0, 0, 0)
		unix.Close(dfd)
		if errno != 0 {
			return fmt.Errorf("landlock_add_rule %s: %w", w, errno)
		}
	}

	// Landlock refuses to restrict a process that could still gain privilege.
	if _, _, errno := unix.Syscall(unix.SYS_PRCTL, unix.PR_SET_NO_NEW_PRIVS, 1, 0); errno != 0 {
		return fmt.Errorf("prctl(NO_NEW_PRIVS): %w", errno)
	}
	if _, _, errno := unix.Syscall(unix.SYS_LANDLOCK_RESTRICT_SELF, fd, 0, 0); errno != 0 {
		return fmt.Errorf("landlock_restrict_self: %w", errno)
	}
	return nil
}
