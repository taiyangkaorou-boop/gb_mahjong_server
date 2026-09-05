#ifndef GBMJ_CGBM_H
#define GBMJ_CGBM_H

#ifdef __cplusplus
extern "C" {
#endif

#define GBMJ_OK 0
#define GBMJ_ERR_PARSE -1
#define GBMJ_ERR_INTERNAL -2

typedef struct {
    int tot_fan;
    int fan_ids[32];
    int fan_scores[32];
    int n_fan;
} gbmj_fan_result;

/* hand_str 为 zheng-fan/GB-Mahjong 手牌字符串。 */
int gbmj_judge_hu(const char *hand_str, int *out_hu);
int gbmj_count_fan(const char *hand_str, gbmj_fan_result *out);
int gbmj_calc_ting(const char *hand_str, unsigned char *out_lib_ids, int cap, int *n);

#ifdef __cplusplus
}
#endif

#endif
