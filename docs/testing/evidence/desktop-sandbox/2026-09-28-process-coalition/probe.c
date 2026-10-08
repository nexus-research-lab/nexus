// Isolated macOS supervision experiment; never signals a PID supplied by discovery.
#include <dlfcn.h>
#include <errno.h>
#include <inttypes.h>
#include <libproc.h>
#include <mach/mach.h>
#include <signal.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/wait.h>
#include <unistd.h>

// Fixed XNU f6217f891ac0bb64f3d375211650a4c1ff8ca1ea private observation ABI.
struct coalition_ids { uint64_t ids[2]; uint64_t reserved[3]; };
typedef int (*usage_fn)(uint64_t, void *, size_t);
typedef int (*signal_fn)(audit_token_t *, int);

static kern_return_t read_audit(int pid, audit_token_t *token) {
    mach_port_t port = MACH_PORT_NULL;
    kern_return_t result = task_name_for_pid(mach_task_self(), pid, &port);
    if (result == KERN_SUCCESS) {
        mach_msg_type_number_t count = TASK_AUDIT_TOKEN_COUNT;
        result = task_info(port, TASK_AUDIT_TOKEN, (task_info_t)token, &count);
    }
    if (port != MACH_PORT_NULL) mach_port_deallocate(mach_task_self(), port);
    return result;
}

static void identity(const char *kind) {
    struct coalition_ids ids = {0};
    errno = 0;
    int n = proc_pidinfo(getpid(), 20, 0, &ids, sizeof(ids));
    int err = errno;
    audit_token_t token = {0};
    mach_msg_type_number_t count = TASK_AUDIT_TOKEN_COUNT;
    kern_return_t kr = task_info(mach_task_self(), TASK_AUDIT_TOKEN, (task_info_t)&token, &count);
    printf("{\"kind\":\"%s\",\"pid\":%d,\"sid\":%d,\"coalitionBytes\":%d,\"coalitionErrno\":%d,\"resource\":%" PRIu64 ",\"jetsam\":%" PRIu64 ",\"auditResult\":%d,\"audit\":[", kind, getpid(), getsid(0), n, err, ids.ids[0], ids.ids[1], kr);
    for (int i=0;i<8;i++) printf("%s%u", i ? "," : "", token.val[i]);
    puts("]}");
    fflush(stdout);
}

int main(int argc, char **argv) {
    if (argc == 3 && strcmp(argv[1], "members") == 0) {
        uint64_t wanted = strtoull(argv[2], NULL, 10);
        int count = proc_listallpids(NULL, 0);
        if (count <= 0) return 3;
        int capacity = count + 128;
        int *pids = calloc((size_t)capacity, sizeof(int));
        if (!pids) return 4;
        count = proc_listallpids(pids, capacity * (int)sizeof(int));
        if (count <= 0 || count >= capacity) { free(pids); return 5; }
        printf("{\"members\":[");
        int emitted = 0;
        for (int index=0;index<count;index++) {
            int pid = pids[index];
            struct coalition_ids ids = {0};
            if (pid <= 1 || proc_pidinfo(pid,20,0,&ids,sizeof(ids)) != sizeof(ids) || ids.ids[0] != wanted) continue;
            audit_token_t before = {0}, after = {0};
            if (read_audit(pid,&before) != KERN_SUCCESS) continue;
            if (proc_pidinfo(pid,20,0,&ids,sizeof(ids)) != sizeof(ids) || ids.ids[0] != wanted) continue;
            if (read_audit(pid,&after) != KERN_SUCCESS || memcmp(&before,&after,sizeof(before)) != 0) continue;
            printf("%s{\"pid\":%d,\"resource\":%" PRIu64 ",\"audit\":[",emitted++ ? "," : "",pid,wanted);
            for(int i=0;i<8;i++) printf("%s%u",i ? "," : "",before.val[i]);
            printf("]}");
        }
        puts("]}"); free(pids); return 0;
    }
    if (argc == 2 && strcmp(argv[1], "leaf") == 0) {
        alarm(40); identity("detached"); sleep(35); return 0;
    }
    if (argc == 3 && strcmp(argv[1], "inspect") == 0) {
        int pid = atoi(argv[2]);
        mach_port_t name = MACH_PORT_NULL;
        kern_return_t nr = task_name_for_pid(mach_task_self(), pid, &name);
        audit_token_t token = {0};
        mach_msg_type_number_t count = TASK_AUDIT_TOKEN_COUNT;
        kern_return_t kr = nr == KERN_SUCCESS ? task_info(name, TASK_AUDIT_TOKEN, (task_info_t)&token, &count) : nr;
        struct coalition_ids ids = {0};
        errno = 0;
        int n = proc_pidinfo(pid, 20, 0, &ids, sizeof(ids));
        printf("{\"pid\":%d,\"taskNameResult\":%d,\"auditResult\":%d,\"coalitionBytes\":%d,\"resource\":%" PRIu64 ",\"audit\":[", pid, nr, kr, n, ids.ids[0]);
        for(int i=0;i<8;i++) printf("%s%u",i ? "," : "",token.val[i]);
        puts("]}");
        if (name != MACH_PORT_NULL) mach_port_deallocate(mach_task_self(), name);
        return 0;
    }
    if (argc == 2 && (strcmp(argv[1], "worker") == 0 || strcmp(argv[1], "worker-exec") == 0)) {
        alarm(45);
        identity("root");
        pid_t child = fork();
        if (child < 0) return 2;
        if (child == 0) {
            alarm(40);
            errno = 0;
            int result = setsid();
            printf("{\"kind\":\"setsid\",\"result\":%d,\"errno\":%d}\n", result, errno);
            fflush(stdout);
            if (strcmp(argv[1], "worker-exec") == 0) {
                pid_t grandchild=fork();
                if (grandchild < 0) _exit(3);
                if (grandchild > 0) _exit(0);
                execl(argv[0],argv[0],"leaf",(char *)NULL);
                _exit(4);
            }
            identity("detached");
            sleep(35);
            _exit(0);
        }
        sleep(1);
        return 0;
    }
    if (argc == 3 && strcmp(argv[1], "usage") == 0) {
        usage_fn usage = (usage_fn)dlsym(RTLD_DEFAULT, "coalition_info_resource_usage");
        if (!usage) { puts("{\"available\":false}"); return 0; }
        uint64_t values[2] = {0};
        errno = 0;
        int result = usage(strtoull(argv[2], NULL, 10), values, sizeof(values));
        printf("{\"available\":true,\"result\":%d,\"errno\":%d,\"started\":%" PRIu64 ",\"exited\":%" PRIu64 "}\n", result, errno, values[0], values[1]);
        return 0;
    }
    if (argc == 10 && strcmp(argv[1], "signal") == 0) {
        signal_fn send = (signal_fn)dlsym(RTLD_DEFAULT, "proc_signal_with_audittoken");
        if (!send) { puts("{\"available\":false}"); return 0; }
        audit_token_t token = {0};
        for(int i=0;i<8;i++) token.val[i]=(uint32_t)strtoul(argv[i+2], NULL, 10);
        errno = 0;
        int result = send(&token, SIGTERM);
        printf("{\"available\":true,\"result\":%d,\"errno\":%d}\n",result,errno);
        return 0;
    }
    identity("observer");
    return 0;
}
