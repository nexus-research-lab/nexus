#include <EndpointSecurity/EndpointSecurity.h>
#include <stdio.h>

int main(void) {
    if (__builtin_available(macOS 27.0, *)) {
        es_client_t *client = NULL;
        es_new_client_result_t result = es_new_descendants_client(&client,
            ^(es_client_t *unused_client, const es_message_t *unused_message) {
                (void)unused_client;
                (void)unused_message;
            });
        /* No subscriptions, changes to permissions or auth responses. */
        printf("{\"apiAvailable\":true,\"result\":%d,\"notEntitled\":%s,\"subscribed\":false}\n",
               result, result == ES_NEW_CLIENT_RESULT_ERR_NOT_ENTITLED ? "true" : "false");
        if (result == ES_NEW_CLIENT_RESULT_SUCCESS) es_delete_client(client);
    } else {
        puts("{\"apiAvailable\":false,\"subscribed\":false}");
    }
    return 0;
}
