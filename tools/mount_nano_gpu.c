#define _GNU_SOURCE
#include <fcntl.h>
#include <sched.h>
#include <unistd.h>
#include <stdlib.h>
#include <stdio.h>
#include <sys/mount.h>
#include <string.h>
#include <errno.h>
#include <time.h>
#include <sys/timeb.h>
#include <sys/types.h>
#include <sys/stat.h>

int usage()
{
	printf("mount_sgpu pid sgpuSrc sgpuDst ctlSrc ctlDst\n");
	return 0;
}

int main(int argc, char* argv[])
{
	int ret = 0;
	char mnt_path[256];
	char *pid = NULL;
	char *sgpu_src = NULL, *sgpu_dst = NULL;
	char *ctl_src = NULL, *ctl_dst = NULL;

	if (argc < 5 || !strcmp(argv[1], "-h")) {
		return usage();
	}

	// 1. get pid/sgpu path/sgpuctl path
	pid = argv[1];
	sgpu_src = argv[2];
	sgpu_dst = argv[3];
	ctl_src = argv[4]; // should be /dev/nvidia0
	ctl_dst = argv[5]; // shoule be /dev/nvidiactl

	// 2. enter namespace
	sprintf(mnt_path, "/proc/%s/ns/mnt", pid);
	int fd = open(mnt_path, O_RDONLY | O_CLOEXEC);
	if (fd < 0) {
		printf("Can not open mnt path:%s\n", mnt_path);
		return -ENOMEM;
	}

	printf("entered namespace :%s\n", mnt_path);

	if (setns(fd, 0) < 0) {
		printf("set ns failed\n");
		return -ENOMEM;
	}

	// 3. show mount time
	/* Example of timestamp in millisecond. */
	struct timeb timer_msec;
	long long int timestamp_msec; /* timestamp in millisecond. */
	if (!ftime(&timer_msec)) {
		timestamp_msec = ((long long int)timer_msec.time) * 1000ll + (long long int)timer_msec.millitm;
	} else {
		timestamp_msec = -1;
	}
	printf("%lld milliseconds since epoch\n", timestamp_msec);

    int gpu_holder = open(sgpu_dst, O_CREAT | O_RDONLY, 0755);
    int ctl_holder = open(ctl_dst, O_CREAT | O_RDONLY, 0755);
    printf("gpu_holder:%d\n",gpu_holder);
    printf("ctl_holder:%d\n",ctl_holder);
    close(gpu_holder);
    close(ctl_holder);
	// 3. mount
	if (ret = mount(sgpu_src, sgpu_dst, NULL, MS_BIND, NULL) < 0) {
		printf("mount from %s to %s failed:%d\n", sgpu_src, sgpu_dst, errno);
		return ret;
	}

	if (ret = mount(ctl_src, ctl_dst, NULL, MS_BIND, NULL) < 0) {
		printf("mount from %s to %s failed\n", ctl_src, ctl_dst);
		return ret;
	}

	printf("Successfully bind mount for %s-->%s and %s-->%s\n", sgpu_src,sgpu_dst,ctl_src,ctl_dst);
	return 0;
}