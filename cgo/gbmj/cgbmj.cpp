#include "cgbmj.h"

#include "fan.h"
#include "handtiles.h"
#include "tile.h"

#include <cstring>
#include <stdexcept>
#include <vector>

extern "C" {

int gbmj_judge_hu(const char *hand_str, int *out_hu) {
    if (hand_str == nullptr || out_hu == nullptr) {
        return GBMJ_ERR_PARSE;
    }
    try {
        mahjong::Handtiles ht;
        if (ht.StringToHandtiles(hand_str) != 0) {
            return GBMJ_ERR_PARSE;
        }
        mahjong::Fan fan;
        *out_hu = fan.JudgeHu(ht) ? 1 : 0;
        return GBMJ_OK;
    } catch (...) {
        return GBMJ_ERR_INTERNAL;
    }
}

int gbmj_count_fan(const char *hand_str, gbmj_fan_result *out) {
    if (hand_str == nullptr || out == nullptr) {
        return GBMJ_ERR_PARSE;
    }
    try {
        mahjong::Handtiles ht;
        if (ht.StringToHandtiles(hand_str) != 0) {
            return GBMJ_ERR_PARSE;
        }
        mahjong::Fan fan;
        fan.CountFan(ht);
        std::memset(out, 0, sizeof(*out));
        out->tot_fan = fan.tot_fan_res;
        int n = 0;
        for (int i = 1; i < mahjong::FAN_SIZE && n < 32; i++) {
            for (size_t j = 0; j < fan.fan_table_res[i].size() && n < 32; j++) {
                out->fan_ids[n] = i;
                out->fan_scores[n] = mahjong::FAN_SCORE[i];
                n++;
            }
        }
        out->n_fan = n;
        return GBMJ_OK;
    } catch (...) {
        return GBMJ_ERR_INTERNAL;
    }
}

int gbmj_calc_ting(const char *hand_str, unsigned char *out_lib_ids, int cap, int *n) {
    if (hand_str == nullptr || n == nullptr) {
        return GBMJ_ERR_PARSE;
    }
    try {
        mahjong::Handtiles ht;
        if (ht.StringToHandtiles(hand_str) != 0) {
            return GBMJ_ERR_PARSE;
        }
        mahjong::Fan fan;
        std::vector<mahjong::Tile> ting = fan.CalcTing(ht);
        int w = 0;
        for (size_t i = 0; i < ting.size(); i++) {
            int id = ting[i].GetId();
            // 库会扫 1..42；花牌不能进听口，只保留序数/字牌 1..34。
            if (id < 1 || id > 34) {
                continue;
            }
            if (out_lib_ids != nullptr && w < cap) {
                out_lib_ids[w] = static_cast<unsigned char>(id);
            }
            w++;
        }
        *n = w;
        return GBMJ_OK;
    } catch (...) {
        return GBMJ_ERR_INTERNAL;
    }
}

} // extern "C"
