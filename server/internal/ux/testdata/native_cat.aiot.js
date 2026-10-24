export default function(global, globalThis, window, $app_exports$, $app_evaluate$) {
    var org_app_require = $app_require$;
    (function(global, globalThis, window, $app_exports$, $app_evaluate$) {
        var setTimeout = global.setTimeout;
        var setInterval = global.setInterval;
        var clearTimeout = global.clearTimeout;
        var clearInterval = global.clearInterval;
        var $app_require$1 = global.$app_require$ || org_app_require;
        var createPageHandler = function() {
            return (()=>{
                var __webpack_modules__ = {
                    "./src/common/demo.js" (__unused_rspack_module, exports) {
                        "use strict";
                        Object.defineProperty(exports, "__esModule", {
                            value: true
                        });
                        exports["default"] = void 0;
                        var _default = exports["default"] = {
                            "agent-cat:config": {
                                skin: "gray",
                                quota: "claude:7d",
                                impatientMin: 3
                            },
                            "agent-cat:agents": [
                                {
                                    id: "w1-p1",
                                    agent: "codex",
                                    project: "blog",
                                    title: "修登录页的样式",
                                    status: "working",
                                    seq: 1000
                                },
                                {
                                    id: "w1-p2",
                                    agent: "claude",
                                    project: "shop",
                                    title: "排查内存占用",
                                    status: "blocked",
                                    seq: 999
                                },
                                {
                                    id: "w1-p3",
                                    agent: "claude",
                                    project: "api",
                                    title: "补单元测试",
                                    status: "done",
                                    seq: 998
                                },
                                {
                                    id: "w1-p4",
                                    agent: "codex",
                                    project: "web",
                                    title: "升级依赖",
                                    status: "working",
                                    seq: 997
                                },
                                {
                                    id: "w1-p5",
                                    agent: "claude",
                                    project: "cli",
                                    title: "优化首屏加载",
                                    status: "done",
                                    seq: 996
                                },
                                {
                                    id: "w1-p6",
                                    agent: "claude",
                                    project: "docs",
                                    title: "补接口测试",
                                    status: "working",
                                    seq: 995
                                },
                                {
                                    id: "w1-p7",
                                    agent: "codex",
                                    project: "infra",
                                    title: "加缓存",
                                    status: "idle",
                                    seq: 994
                                },
                                {
                                    id: "w1-p8",
                                    agent: "claude",
                                    project: "mobile",
                                    title: "迁移数据库字段",
                                    status: "idle",
                                    seq: 993
                                },
                                {
                                    id: "w2-p1",
                                    agent: "claude",
                                    project: "search",
                                    title: "写发布说明",
                                    status: "idle",
                                    seq: 992
                                },
                                {
                                    id: "w2-p2",
                                    agent: "codex",
                                    project: "billing",
                                    title: "修复时区问题",
                                    status: "idle",
                                    seq: 991
                                },
                                {
                                    id: "w2-p3",
                                    agent: "claude",
                                    project: "auth",
                                    title: "重构配置加载",
                                    status: "idle",
                                    seq: 990
                                },
                                {
                                    id: "w2-p4",
                                    agent: "claude",
                                    project: "chat",
                                    title: "整理日志格式",
                                    status: "idle",
                                    seq: 989
                                },
                                {
                                    id: "w2-p5",
                                    agent: "codex",
                                    project: "blog",
                                    title: "修登录页的样式",
                                    status: "idle",
                                    seq: 988
                                },
                                {
                                    id: "w2-p6",
                                    agent: "claude",
                                    project: "shop",
                                    title: "排查内存占用",
                                    status: "idle",
                                    seq: 987
                                },
                                {
                                    id: "w2-p7",
                                    agent: "claude",
                                    project: "api",
                                    title: "补单元测试",
                                    status: "idle",
                                    seq: 986
                                },
                                {
                                    id: "w2-p8",
                                    agent: "codex",
                                    project: "web",
                                    title: "升级依赖",
                                    status: "idle",
                                    seq: 985
                                },
                                {
                                    id: "w3-p1",
                                    agent: "claude",
                                    project: "cli",
                                    title: "优化首屏加载",
                                    status: "idle",
                                    seq: 984
                                },
                                {
                                    id: "w3-p2",
                                    agent: "claude",
                                    project: "docs",
                                    title: "补接口测试",
                                    status: "idle",
                                    seq: 983
                                },
                                {
                                    id: "w3-p3",
                                    agent: "codex",
                                    project: "infra",
                                    title: "加缓存",
                                    status: "idle",
                                    seq: 982
                                },
                                {
                                    id: "w3-p4",
                                    agent: "claude",
                                    project: "mobile",
                                    title: "迁移数据库字段",
                                    status: "idle",
                                    seq: 981
                                },
                                {
                                    id: "w3-p5",
                                    agent: "claude",
                                    project: "search",
                                    title: "写发布说明",
                                    status: "idle",
                                    seq: 980
                                },
                                {
                                    id: "w3-p6",
                                    agent: "codex",
                                    project: "billing",
                                    title: "修复时区问题",
                                    status: "idle",
                                    seq: 979
                                },
                                {
                                    id: "w3-p7",
                                    agent: "claude",
                                    project: "auth",
                                    title: "重构配置加载",
                                    status: "idle",
                                    seq: 978
                                },
                                {
                                    id: "w3-p8",
                                    agent: "claude",
                                    project: "chat",
                                    title: "整理日志格式",
                                    status: "idle",
                                    seq: 977
                                },
                                {
                                    id: "w4-p1",
                                    agent: "codex",
                                    project: "blog",
                                    title: "修登录页的样式",
                                    status: "idle",
                                    seq: 976
                                },
                                {
                                    id: "w4-p2",
                                    agent: "claude",
                                    project: "shop",
                                    title: "排查内存占用",
                                    status: "idle",
                                    seq: 975
                                },
                                {
                                    id: "w4-p3",
                                    agent: "claude",
                                    project: "api",
                                    title: "补单元测试",
                                    status: "idle",
                                    seq: 974
                                },
                                {
                                    id: "w4-p4",
                                    agent: "codex",
                                    project: "web",
                                    title: "升级依赖",
                                    status: "idle",
                                    seq: 973
                                },
                                {
                                    id: "w4-p5",
                                    agent: "claude",
                                    project: "cli",
                                    title: "优化首屏加载",
                                    status: "idle",
                                    seq: 972
                                },
                                {
                                    id: "w4-p6",
                                    agent: "claude",
                                    project: "docs",
                                    title: "补接口测试",
                                    status: "idle",
                                    seq: 971
                                },
                                {
                                    id: "w4-p7",
                                    agent: "codex",
                                    project: "infra",
                                    title: "加缓存",
                                    status: "idle",
                                    seq: 970
                                },
                                {
                                    id: "w4-p8",
                                    agent: "claude",
                                    project: "mobile",
                                    title: "迁移数据库字段",
                                    status: "idle",
                                    seq: 969
                                },
                                {
                                    id: "w5-p1",
                                    agent: "claude",
                                    project: "search",
                                    title: "写发布说明",
                                    status: "idle",
                                    seq: 968
                                },
                                {
                                    id: "w5-p2",
                                    agent: "codex",
                                    project: "billing",
                                    title: "修复时区问题",
                                    status: "idle",
                                    seq: 967
                                },
                                {
                                    id: "w5-p3",
                                    agent: "claude",
                                    project: "auth",
                                    title: "重构配置加载",
                                    status: "idle",
                                    seq: 966
                                },
                                {
                                    id: "w5-p4",
                                    agent: "claude",
                                    project: "chat",
                                    title: "整理日志格式",
                                    status: "idle",
                                    seq: 965
                                },
                                {
                                    id: "w5-p5",
                                    agent: "codex",
                                    project: "blog",
                                    title: "修登录页的样式",
                                    status: "idle",
                                    seq: 964
                                },
                                {
                                    id: "w5-p6",
                                    agent: "claude",
                                    project: "shop",
                                    title: "排查内存占用",
                                    status: "idle",
                                    seq: 963
                                },
                                {
                                    id: "w5-p7",
                                    agent: "claude",
                                    project: "api",
                                    title: "补单元测试",
                                    status: "idle",
                                    seq: 962
                                },
                                {
                                    id: "w5-p8",
                                    agent: "codex",
                                    project: "web",
                                    title: "升级依赖",
                                    status: "idle",
                                    seq: 961
                                },
                                {
                                    id: "w6-p1",
                                    agent: "claude",
                                    project: "cli",
                                    title: "优化首屏加载",
                                    status: "idle",
                                    seq: 960
                                },
                                {
                                    id: "w6-p2",
                                    agent: "claude",
                                    project: "docs",
                                    title: "补接口测试",
                                    status: "idle",
                                    seq: 959
                                },
                                {
                                    id: "w6-p3",
                                    agent: "codex",
                                    project: "infra",
                                    title: "加缓存",
                                    status: "idle",
                                    seq: 958
                                },
                                {
                                    id: "w6-p4",
                                    agent: "claude",
                                    project: "mobile",
                                    title: "迁移数据库字段",
                                    status: "idle",
                                    seq: 957
                                },
                                {
                                    id: "w6-p5",
                                    agent: "claude",
                                    project: "search",
                                    title: "写发布说明",
                                    status: "idle",
                                    seq: 956
                                },
                                {
                                    id: "w6-p6",
                                    agent: "codex",
                                    project: "billing",
                                    title: "修复时区问题",
                                    status: "idle",
                                    seq: 955
                                },
                                {
                                    id: "w6-p7",
                                    agent: "claude",
                                    project: "auth",
                                    title: "重构配置加载",
                                    status: "idle",
                                    seq: 954
                                },
                                {
                                    id: "w6-p8",
                                    agent: "claude",
                                    project: "chat",
                                    title: "整理日志格式",
                                    status: "idle",
                                    seq: 953
                                },
                                {
                                    id: "w7-p1",
                                    agent: "codex",
                                    project: "blog",
                                    title: "修登录页的样式",
                                    status: "idle",
                                    seq: 952
                                }
                            ],
                            "agent-cat:stats": {
                                today: {
                                    done: 51
                                },
                                days: [
                                    {
                                        date: "10-18",
                                        weekday: "一",
                                        done: 0,
                                        usd: 0.0
                                    },
                                    {
                                        date: "10-19",
                                        weekday: "二",
                                        done: 3,
                                        usd: 20.5
                                    },
                                    {
                                        date: "10-20",
                                        weekday: "三",
                                        done: 6,
                                        usd: 41.0
                                    },
                                    {
                                        date: "10-21",
                                        weekday: "四",
                                        done: 9,
                                        usd: 61.5
                                    },
                                    {
                                        date: "10-22",
                                        weekday: "五",
                                        done: 12,
                                        usd: 82.0
                                    },
                                    {
                                        date: "10-23",
                                        weekday: "六",
                                        done: 15,
                                        usd: 102.5
                                    },
                                    {
                                        date: "10-24",
                                        weekday: "日",
                                        done: 18,
                                        usd: 123.0
                                    }
                                ]
                            },
                            "agent-cat:cost": {
                                today: 139.2,
                                days: [
                                    {
                                        date: "10-18",
                                        weekday: "一",
                                        done: 0,
                                        usd: 0.0
                                    },
                                    {
                                        date: "10-19",
                                        weekday: "二",
                                        done: 3,
                                        usd: 20.5
                                    },
                                    {
                                        date: "10-20",
                                        weekday: "三",
                                        done: 6,
                                        usd: 41.0
                                    },
                                    {
                                        date: "10-21",
                                        weekday: "四",
                                        done: 9,
                                        usd: 61.5
                                    },
                                    {
                                        date: "10-22",
                                        weekday: "五",
                                        done: 12,
                                        usd: 82.0
                                    },
                                    {
                                        date: "10-23",
                                        weekday: "六",
                                        done: 15,
                                        usd: 102.5
                                    },
                                    {
                                        date: "10-24",
                                        weekday: "日",
                                        done: 18,
                                        usd: 123.0
                                    }
                                ]
                            },
                            "agent-cat:usage": [
                                {
                                    agent: "claude",
                                    windows: [
                                        {
                                            name: "5h",
                                            used: 54,
                                            resetsAt: 1792840343
                                        },
                                        {
                                            name: "7d",
                                            used: 77,
                                            resetsAt: 1793088743
                                        }
                                    ]
                                }
                            ]
                        };
                    }
                };
                var __webpack_module_cache__ = {};
                function __webpack_require__(moduleId) {
                    var cachedModule = __webpack_module_cache__[moduleId];
                    if (void 0 !== cachedModule) return cachedModule.exports;
                    var module = __webpack_module_cache__[moduleId] = {
                        exports: {}
                    };
                    __webpack_modules__[moduleId](module, module.exports, __webpack_require__);
                    return module.exports;
                }
                (()=>{
                    __webpack_require__.g = (()=>{
                        if ('object' == typeof globalThis) return globalThis;
                        try {
                            return this || new Function('return this')();
                        } catch (e) {
                            if ('object' == typeof window) return window;
                        }
                    })();
                })();
                (()=>{
                    __webpack_require__.rv = ()=>"1.7.12";
                })();
                (()=>{
                    __webpack_require__.ruid = "bundler=rspack@1.7.12";
                })();
                var __webpack_exports__ = {};
                (()=>{
                    var $app_style$ = [
                        [
                            [
                                [
                                    0,
                                    "page"
                                ]
                            ],
                            {
                                width: "212px",
                                flexDirection: "column",
                                backgroundColor: "#000000"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "pager"
                                ]
                            ],
                            {
                                width: "212px"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "spacer"
                                ]
                            ],
                            {
                                flexGrow: 1
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "cat"
                                ]
                            ],
                            {
                                width: "212px",
                                flexDirection: "column",
                                alignItems: "center",
                                paddingTop: "8px",
                                paddingLeft: "10px",
                                paddingRight: "10px",
                                backgroundColor: "#0a0c12"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "headline"
                                ]
                            ],
                            {
                                width: "192px",
                                height: "26px",
                                lines: 1,
                                textOverflow: "ellipsis",
                                fontSize: "18px",
                                fontWeight: "bold",
                                textAlign: "center"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "quota"
                                ]
                            ],
                            {
                                width: "192px",
                                height: "20px",
                                flexDirection: "row",
                                alignItems: "center"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "quota-name"
                                ]
                            ],
                            {
                                width: "64px",
                                fontSize: "12px",
                                color: "#e8ebf1",
                                lines: 1
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "quota-track"
                                ]
                            ],
                            {
                                width: "68px",
                                height: "8px",
                                borderRadius: "4px",
                                backgroundColor: "#2a2e36"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "quota-fill"
                                ]
                            ],
                            {
                                height: "8px",
                                borderRadius: "4px"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "quota-pct"
                                ]
                            ],
                            {
                                width: "60px",
                                fontSize: "12px",
                                textAlign: "right",
                                lines: 1
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "sprite-box"
                                ]
                            ],
                            {
                                width: "192px",
                                height: "142px",
                                marginTop: "2px",
                                justifyContent: "center",
                                alignItems: "center"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "sprite"
                                ]
                            ],
                            {
                                width: "140px",
                                height: "140px"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "bubble"
                                ]
                            ],
                            {
                                width: "192px",
                                height: "62px",
                                marginTop: "6px",
                                paddingTop: "6px",
                                paddingBottom: "6px",
                                paddingLeft: "10px",
                                paddingRight: "10px",
                                borderRadius: "14px",
                                backgroundColor: "#16191f",
                                flexDirection: "column",
                                borderTopWidth: "2px",
                                borderRightWidth: "2px",
                                borderBottomWidth: "2px",
                                borderLeftWidth: "2px"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "bubble-empty"
                                ]
                            ],
                            {
                                width: "192px",
                                height: "62px",
                                marginTop: "6px"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "bubble-row"
                                ]
                            ],
                            {
                                flexDirection: "row",
                                alignItems: "center"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "bubble-project"
                                ]
                            ],
                            {
                                flex: 1,
                                fontSize: "14px",
                                fontWeight: "bold",
                                lines: 1,
                                textOverflow: "ellipsis",
                                color: "#ffffff"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "bubble-count"
                                ]
                            ],
                            {
                                marginLeft: "6px",
                                fontSize: "11px",
                                color: "#8a8f98"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "bubble-title"
                                ]
                            ],
                            {
                                fontSize: "12px",
                                color: "#8a8f98",
                                lines: 1,
                                textOverflow: "ellipsis"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "status"
                                ]
                            ],
                            {
                                width: "192px",
                                height: "22px",
                                marginTop: "8px",
                                flexDirection: "row",
                                alignItems: "center"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "dot-group"
                                ]
                            ],
                            {
                                flexDirection: "row",
                                alignItems: "center",
                                marginRight: "8px"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "dot"
                                ]
                            ],
                            {
                                width: "8px",
                                height: "8px",
                                borderRadius: "4px",
                                marginRight: "4px"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "dot-n"
                                ]
                            ],
                            {
                                fontSize: "12px",
                                color: "#8a8f98"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "tally"
                                ]
                            ],
                            {
                                height: "22px",
                                paddingLeft: "4px",
                                flexDirection: "row",
                                alignItems: "center"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "cost"
                                ]
                            ],
                            {
                                marginRight: "6px",
                                fontSize: "12px"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "fish"
                                ]
                            ],
                            {
                                width: "24px",
                                height: "12px"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "fish-n"
                                ]
                            ],
                            {
                                marginLeft: "4px",
                                fontSize: "12px"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "today"
                                ]
                            ],
                            {
                                width: "212px",
                                flexDirection: "column",
                                paddingTop: "10px",
                                paddingRight: "10px",
                                paddingBottom: "10px",
                                paddingLeft: "10px",
                                backgroundColor: "#0a0c12"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "today-head"
                                ]
                            ],
                            {
                                flexDirection: "row",
                                alignItems: "center"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "today-title"
                                ]
                            ],
                            {
                                fontSize: "18px",
                                fontWeight: "bold",
                                color: "#ffffff"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "today-date"
                                ]
                            ],
                            {
                                fontSize: "11px",
                                color: "#8a8f98"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "blocks"
                                ]
                            ],
                            {
                                flexDirection: "row",
                                alignItems: "center",
                                marginTop: "6px"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "block"
                                ]
                            ],
                            {
                                width: "92px",
                                flexDirection: "column",
                                paddingTop: "4px",
                                paddingBottom: "4px",
                                paddingLeft: "6px",
                                paddingRight: "6px"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "block-row"
                                ]
                            ],
                            {
                                flexDirection: "row",
                                alignItems: "center"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "block-fish"
                                ]
                            ],
                            {
                                width: "28px",
                                height: "14px",
                                marginRight: "6px"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "block-num"
                                ]
                            ],
                            {
                                fontSize: "20px",
                                fontWeight: "bold",
                                lines: 1
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "block-label"
                                ]
                            ],
                            {
                                fontSize: "11px",
                                color: "#8a8f98"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "mark"
                                ]
                            ],
                            {
                                width: "20px",
                                height: "2px",
                                marginTop: "3px",
                                borderRadius: "1px"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "chart"
                                ]
                            ],
                            {
                                width: "192px",
                                marginTop: "8px",
                                flexDirection: "row",
                                justifyContent: "space-between",
                                alignItems: "flex-end"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "bar-col"
                                ]
                            ],
                            {
                                width: "26px",
                                flexDirection: "column",
                                alignItems: "center"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "bar-label"
                                ]
                            ],
                            {
                                fontSize: "9px",
                                lines: 1
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "bar"
                                ]
                            ],
                            {
                                width: "14px",
                                marginTop: "2px",
                                borderRadius: "3px"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "bar-day"
                                ]
                            ],
                            {
                                fontSize: "10px",
                                marginTop: "3px"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "section"
                                ]
                            ],
                            {
                                fontSize: "12px",
                                color: "#8a8f98",
                                marginTop: "10px"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "q-row"
                                ]
                            ],
                            {
                                width: "192px",
                                height: "16px",
                                marginTop: "4px",
                                flexDirection: "row",
                                alignItems: "center"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "q-label"
                                ]
                            ],
                            {
                                width: "66px",
                                fontSize: "11px",
                                color: "#c9ced8",
                                lines: 1
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "q-track"
                                ]
                            ],
                            {
                                width: "62px",
                                height: "6px",
                                borderRadius: "3px",
                                backgroundColor: "#22252c"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "q-fill"
                                ]
                            ],
                            {
                                height: "6px",
                                borderRadius: "3px"
                            }
                        ],
                        [
                            [
                                [
                                    0,
                                    "q-text"
                                ]
                            ],
                            {
                                fontSize: "10px",
                                color: "#8a8f98"
                            }
                        ]
                    ];
                    var $app_script$ = function __scriptModule__(module, exports, $app_require$1) {
                        "use strict";
                        Object.defineProperty(exports, "__esModule", {
                            value: true
                        });
                        exports.default = void 0;
                        var _system = _interopRequireDefault($app_require$1("@app-module/system.interconnect"));
                        var _system2 = _interopRequireDefault($app_require$1("@app-module/system.device"));
                        var _system3 = _interopRequireDefault($app_require$1("@app-module/system.vibrator"));
                        var _demo = _interopRequireDefault(__webpack_require__("./src/common/demo.js"));
                        function _interopRequireDefault(e) {
                            return e && e.__esModule ? e : {
                                default: e
                            };
                        }
                        const PRIORITY = {
                            blocked: 0,
                            done: 1,
                            working: 2,
                            idle: 3,
                            unknown: 4
                        };
                        const COLOR = {
                            blocked: '#ff9f1c',
                            working: '#2ea0ff',
                            done: '#3ddc84',
                            idle: '#5c6068',
                            unknown: '#5c6068'
                        };
                        const HEAD_COLOR = {
                            alert: '#ff9f1c',
                            impatient: '#ff5c5c',
                            work: '#2ea0ff',
                            celebrate: '#3ddc84',
                            sleep: '#c9ced8'
                        };
                        const FRAMES = {
                            work: 6,
                            alert: 6,
                            celebrate: 6,
                            sleep: 8
                        };
                        const ACTION = {
                            alert: [
                                'alert',
                                2800
                            ],
                            impatient: [
                                'alert',
                                1200
                            ],
                            work: [
                                'work',
                                3600
                            ],
                            celebrate: [
                                'celebrate',
                                3200
                            ],
                            sleep: [
                                'sleep',
                                9600
                            ]
                        };
                        const METRIC = {
                            done: {
                                today: '#e29640',
                                other: '#5c4a3a'
                            },
                            usd: {
                                today: '#6fb6ff',
                                other: '#2b4060'
                            }
                        };
                        const WEEKDAYS = [
                            '日',
                            '一',
                            '二',
                            '三',
                            '四',
                            '五',
                            '六'
                        ];
                        const CELEBRATE_MS = 8000;
                        const CHART_H = 40;
                        const S = 'agent-cat:';
                        function fmtUsd(v) {
                            if (v < 100) return '$' + v.toFixed(2);
                            if (v < 1000) return '$' + v.toFixed(1);
                            return '$' + Math.round(v);
                        }
                        function fmtUsdShort(v) {
                            if (v < 10) return '$' + v.toFixed(1);
                            if (v < 1000) return '$' + Math.round(v);
                            return '$' + (v / 1000).toFixed(1) + 'k';
                        }
                        function fmtReset(sec) {
                            const left = Math.max(0, sec - Date.now() / 1000);
                            if (left >= 86400) return Math.floor(left / 86400) + ' 天';
                            const h = Math.floor(left / 3600);
                            const m = Math.floor(left % 3600 / 60);
                            return h + ':' + (m < 10 ? '0' : '') + m;
                        }
                        function usedColor(u) {
                            return u >= 80 ? '#ff6b6b' : u >= 50 ? '#ff9f1c' : '#3ddc84';
                        }
                        function byProjectThenId(x, y) {
                            return (x.project || '').localeCompare(y.project || '') || (x.id < y.id ? -1 : x.id > y.id ? 1 : 0);
                        }
                        var _default = exports.default = {
                            private: {
                                pageH: 303,
                                headline: '都在休息 zZ',
                                headColor: '#c9ced8',
                                quota: {
                                    show: false,
                                    name: '',
                                    w: 4,
                                    color: '#3ddc84',
                                    text: ''
                                },
                                frame: '/common/pet/gray/sleep_0.png',
                                bubble: {
                                    show: false,
                                    project: '',
                                    title: '',
                                    count: '',
                                    color: '#5c6068'
                                },
                                counts: {
                                    blocked: 0,
                                    working: 0,
                                    done: 0,
                                    idle: 0
                                },
                                costText: '',
                                costColor: '#5c6068',
                                costBig: '$0.00',
                                fish: 0,
                                fishColor: '#5c6068',
                                dateText: '',
                                metric: 'done',
                                bars: [],
                                quotas: []
                            },
                            onInit () {
                                this.stores = Object.assign({}, _demo.default);
                                this.prevStatus = null;
                                this.focusId = '';
                                this.pool = [];
                                this.prevStatus = null;
                                this.celebrateUntil = 0;
                                this.warned = {};
                                this.anim = null;
                                this.page = 0;
                                this.heard = false;
                                this.conn = _system.default.instance();
                                this.tick = setInterval(()=>this.refresh('tick'), 20000);
                            },
                            onShow () {
                                _system2.default.getInfo({
                                    success: (info)=>{
                                        const w = info.screenWidth || 0;
                                        const h = info.screenHeight || 0;
                                        if (w > 0 && h > 0) this.pageH = Math.round(h / (w / 212));
                                    }
                                });
                                this.refresh('show');
                            },
                            onHide () {
                                this.stopAnim();
                            },
                            onDestroy () {
                                this.stopAnim();
                                clearInterval(this.tick);
                            },
                            send (obj) {
                                try {
                                    this.conn.send({
                                        data: obj,
                                        success: ()=>{},
                                        fail: ()=>{}
                                    });
                                } catch (e) {}
                            },
                            log (msg) {
                                this.send({
                                    t: 'log',
                                    p: '',
                                    level: 'info',
                                    msg: msg
                                });
                            },
                            refresh (what) {
                                const st = this.stores;
                                const agents = st[S + 'agents'] || [];
                                const config = st[S + 'config'] || {};
                                const skin = config.skin || 'orange';
                                const now = Date.now();
                                if (what === S + 'agents') {
                                    if (this.prevStatus) {
                                        for (const a of agents){
                                            const b = this.prevStatus[a.id];
                                            if ('working' === b && ('done' === a.status || 'idle' === a.status)) this.celebrateUntil = now + CELEBRATE_MS;
                                        }
                                    }
                                    this.prevStatus = {};
                                    for (const a of agents)this.prevStatus[a.id] = a.status;
                                    if (this.celebrateUntil > now) setTimeout(()=>this.refresh('celebrate'), CELEBRATE_MS + 50);
                                }
                                const n = {
                                    blocked: 0,
                                    working: 0,
                                    done: 0,
                                    idle: 0
                                };
                                for (const a of agents)if (void 0 !== n[a.status]) n[a.status]++;
                                const limit = null == config.impatientMin ? 5 : config.impatientMin;
                                let worst = null;
                                let mins = 0;
                                if (limit > 0) {
                                    for (const a of agents){
                                        if ('blocked' !== a.status || !a.since) continue;
                                        const m = Math.floor((now - a.since) / 60000);
                                        if (m >= limit && m > mins) {
                                            worst = a;
                                            mins = m;
                                        }
                                    }
                                }
                                if (worst && !this.warned[worst.id + ':' + worst.since]) {
                                    this.warned[worst.id + ':' + worst.since] = true;
                                    _system3.default.vibrate({
                                        mode: 'long'
                                    });
                                }
                                let mood = 'sleep';
                                let headline = n.done > 0 ? '都完成啦' : '都在休息 zZ';
                                if (worst) {
                                    mood = 'impatient';
                                    headline = (worst.project || worst.agent) + ' 等了 ' + mins + ' 分钟！';
                                } else if (n.blocked > 0) {
                                    mood = 'alert';
                                    headline = n.blocked + ' 个等你处理！';
                                } else if (this.celebrateUntil > now) {
                                    mood = 'celebrate';
                                    headline = '搞定一个！';
                                } else if (n.working > 0) {
                                    mood = 'work';
                                    headline = n.working + ' 个在忙';
                                }
                                this.headline = headline;
                                this.headColor = HEAD_COLOR[mood];
                                this.counts.blocked = n.blocked;
                                this.counts.working = n.working;
                                this.counts.done = n.done;
                                this.counts.idle = n.idle;
                                this.startAnim(skin, ACTION[mood][0], ACTION[mood][1]);
                                const sorted = agents.slice().sort((x, y)=>(void 0 === PRIORITY[x.status] ? 4 : PRIORITY[x.status]) - (void 0 === PRIORITY[y.status] ? 4 : PRIORITY[y.status]) || (y.seq || 0) - (x.seq || 0) || byProjectThenId(x, y));
                                const top = sorted.filter((a)=>'idle' !== a.status && 'unknown' !== a.status);
                                this.pool = top.length > 0 ? top : sorted;
                                this.showBubble();
                                const which = config.quota || 'auto';
                                let best = null;
                                if ('off' !== which) {
                                    const parts = 'auto' !== which ? which.split(':') : [];
                                    for (const src of st[S + 'usage'] || [])if (!parts[0] || src.agent === parts[0]) {
                                        for (const w of src.windows || [])if (!parts[1] || w.name === parts[1]) {
                                            if (!best || w.used > best.used) best = Object.assign({
                                                agent: 'claude' === src.agent ? 'Claude' : 'codex' === src.agent ? 'Codex' : src.agent
                                            }, w);
                                        }
                                    }
                                }
                                if (best) {
                                    const pct = Math.min(100, Math.max(0, best.used));
                                    this.quota.show = true;
                                    this.quota.name = best.agent + ' ' + best.name;
                                    this.quota.w = Math.max(4, Math.round(pct / 100 * 68));
                                    this.quota.color = usedColor(best.used);
                                    this.quota.text = Math.round(best.used) + '%' + (best.resetsAt ? ' ' + fmtReset(best.resetsAt) : '');
                                } else this.quota.show = false;
                                const stats = st[S + 'stats'] || {
                                    today: {
                                        done: 0
                                    },
                                    days: []
                                };
                                const cost = st[S + 'cost'];
                                this.fish = stats.today.done;
                                this.fishColor = stats.today.done > 0 ? '#e29640' : '#5c6068';
                                this.costText = cost ? fmtUsdShort(cost.today) : '';
                                this.costColor = cost && cost.today > 0 ? '#6fb6ff' : '#5c6068';
                                this.costBig = cost ? fmtUsd(cost.today) : '$0.00';
                                const d = new Date();
                                this.dateText = d.getMonth() + 1 + '月' + d.getDate() + '日 周' + WEEKDAYS[d.getDay()];
                                this.drawChart();
                                const qs = [];
                                for (const src of st[S + 'usage'] || []){
                                    for (const w of src.windows || [])qs.push({
                                        label: ('claude' === src.agent ? 'Claude' : 'codex' === src.agent ? 'Codex' : src.agent) + ' ' + w.name,
                                        w: Math.max(2, Math.round(Math.min(100, Math.max(0, w.used)) / 100 * 62)),
                                        color: usedColor(w.used),
                                        text: Math.round(w.used) + '%' + (w.resetsAt ? ' ' + fmtReset(w.resetsAt) : '')
                                    });
                                }
                                this.quotas = qs.slice(0, 4);
                            },
                            showBubble () {
                                const pool = this.pool;
                                const index = Math.max(0, pool.findIndex((a)=>a.id === this.focusId));
                                const f = pool[index];
                                if (!f) {
                                    this.bubble.show = false;
                                    return;
                                }
                                this.bubble.show = true;
                                this.bubble.project = f.project || f.agent || '?';
                                this.bubble.title = f.title || '';
                                this.bubble.count = pool.length > 1 ? index + 1 + '/' + pool.length : '';
                                this.bubble.color = COLOR[f.status] || COLOR.unknown;
                            },
                            drawChart () {
                                const st = this.stores;
                                const stats = st[S + 'stats'] || {
                                    days: []
                                };
                                const cost = st[S + 'cost'];
                                const metric = cost ? this.metric : 'done';
                                const days = 'usd' === metric ? cost.days.map((x)=>({
                                        date: x.date,
                                        weekday: x.weekday,
                                        value: x.usd
                                    })) : (stats.days || []).map((x)=>({
                                        date: x.date,
                                        weekday: x.weekday,
                                        value: x.done
                                    }));
                                const m = METRIC[metric];
                                let topV = 'usd' === metric ? 0.01 : 1;
                                for (const x of days)if (x.value > topV) topV = x.value;
                                this.bars = days.map((x, i)=>{
                                    const today = i === days.length - 1;
                                    return {
                                        date: x.date,
                                        weekday: x.weekday,
                                        label: x.value > 0 ? 'usd' === metric ? fmtUsdShort(x.value) : String(x.value) : '',
                                        labelColor: today ? m.today : '#8a8f98',
                                        h: Math.max(2, Math.round(x.value / topV * CHART_H)),
                                        color: today ? m.today : m.other,
                                        dayColor: today ? '#ffffff' : '#5c6068'
                                    };
                                });
                            },
                            startAnim (skin, action, duration) {
                                const key = skin + '/' + action + '/' + duration;
                                if (this.anim && this.anim.key === key) return;
                                this.stopAnim();
                                const n = FRAMES[action];
                                let i = 0;
                                this.frame = '/common/pet/' + skin + '/' + action + '_0.png';
                                this.anim = {
                                    key,
                                    timer: setInterval(()=>{
                                        i = (i + 1) % n;
                                        this.frame = '/common/pet/' + skin + '/' + action + '_' + i + '.png';
                                    }, Math.max(50, Math.round(duration / n)))
                                };
                            },
                            stopAnim () {
                                if (this.anim) clearInterval(this.anim.timer);
                                this.anim = null;
                            },
                            timed (name, fn) {
                                const t0 = Date.now();
                                fn();
                                const t1 = Date.now();
                                setTimeout(()=>this.log('原生 ' + name + ' ' + (Date.now() - t0) + 'ms：JS ' + (t1 - t0) + ' 框架 ' + (Date.now() - t1)), 0);
                            },
                            next () {
                                this.timed('点气泡', ()=>{
                                    const pool = this.pool;
                                    if (pool.length < 2) return;
                                    const index = Math.max(0, pool.findIndex((a)=>a.id === this.focusId));
                                    this.focusId = pool[(index + 1) % pool.length].id;
                                    this.showBubble();
                                });
                            },
                            pick (m) {
                                this.timed('切' + ('usd' === m ? '钱数' : '小鱼干'), ()=>{
                                    this.metric = m;
                                    this.drawChart();
                                });
                            },
                            openToday () {
                                try {
                                    this.$element('pager').swipeTo({
                                        index: 1
                                    });
                                } catch (e) {}
                            },
                            pageChange (evt) {
                                this.page = evt.index;
                            }
                        };
                        const moduleOwn = exports.default || module.exports;
                        const accessors = [
                            'public',
                            'protected',
                            'private'
                        ];
                        if (moduleOwn.data && accessors.some(function(acc) {
                            return moduleOwn[acc];
                        })) throw new Error('页面VM对象中的属性data不可与"' + accessors.join(',') + '"同时存在，请使用private替换data名称');
                        if (!moduleOwn.data) {
                            moduleOwn.data = {};
                            moduleOwn._descriptor = {};
                            accessors.forEach(function(acc) {
                                const accType = typeof moduleOwn[acc];
                                if ('object' === accType) {
                                    moduleOwn.data = Object.assign(moduleOwn.data, moduleOwn[acc]);
                                    for(const name in moduleOwn[acc])moduleOwn._descriptor[name] = {
                                        access: acc
                                    };
                                } else if ('function' === accType) console.warn('页面VM对象中的属性' + acc + '的值不能是函数，请使用对象');
                            });
                        }
                    };
                    var $app_template$ = function(vm) {
                        const _vm_ = vm || this;
                        return aiot.__ce__("div", {
                            __vm__: _vm_,
                            __opts__: {
                                classList: [
                                    "page"
                                ],
                                style: function() {
                                    return __webpack_require__.g.$translateStyle$("height: " + _vm_.pageH + "px");
                                }
                            }
                        }, [
                            aiot.__ce__("swiper", {
                                __vm__: _vm_,
                                __opts__: {
                                    id: "pager",
                                    classList: [
                                        "pager"
                                    ],
                                    style: function() {
                                        return __webpack_require__.g.$translateStyle$("height: " + _vm_.pageH + "px");
                                    },
                                    vertical: "true",
                                    loop: "false",
                                    indicator: "false",
                                    events: {
                                        change: function(evt) {
                                            return _vm_.pageChange(evt);
                                        }
                                    }
                                }
                            }, [
                                aiot.__ce__("div", {
                                    __vm__: _vm_,
                                    __opts__: {
                                        classList: [
                                            "cat"
                                        ],
                                        style: function() {
                                            return __webpack_require__.g.$translateStyle$("height: " + _vm_.pageH + "px");
                                        }
                                    }
                                }, [
                                    aiot.__ce__("text", {
                                        __vm__: _vm_,
                                        __opts__: {
                                            classList: [
                                                "headline"
                                            ],
                                            style: function() {
                                                return __webpack_require__.g.$translateStyle$("color: " + _vm_.headColor);
                                            },
                                            value: function() {
                                                return _vm_.headline;
                                            }
                                        }
                                    }, []),
                                    aiot.__ce__("div", {
                                        __vm__: _vm_,
                                        __opts__: {
                                            classList: [
                                                "quota"
                                            ]
                                        }
                                    }, [
                                        aiot.__ci__({
                                            __vm__: _vm_,
                                            __opts__: {
                                                shown: function() {
                                                    return _vm_.quota.show;
                                                }
                                            }
                                        }, function() {
                                            return [
                                                aiot.__ce__("text", {
                                                    __vm__: _vm_,
                                                    __opts__: {
                                                        classList: [
                                                            "quota-name"
                                                        ],
                                                        value: function() {
                                                            return _vm_.quota.name;
                                                        }
                                                    }
                                                }, []),
                                                aiot.__ce__("div", {
                                                    __vm__: _vm_,
                                                    __opts__: {
                                                        classList: [
                                                            "quota-track"
                                                        ]
                                                    }
                                                }, [
                                                    aiot.__ce__("div", {
                                                        __vm__: _vm_,
                                                        __opts__: {
                                                            classList: [
                                                                "quota-fill"
                                                            ],
                                                            style: function() {
                                                                return __webpack_require__.g.$translateStyle$("width: " + _vm_.quota.w + "px; background-color: " + _vm_.quota.color);
                                                            }
                                                        }
                                                    }, [])
                                                ]),
                                                aiot.__ce__("text", {
                                                    __vm__: _vm_,
                                                    __opts__: {
                                                        classList: [
                                                            "quota-pct"
                                                        ],
                                                        style: function() {
                                                            return __webpack_require__.g.$translateStyle$("color: " + _vm_.quota.color);
                                                        },
                                                        value: function() {
                                                            return _vm_.quota.text;
                                                        }
                                                    }
                                                }, [])
                                            ];
                                        })
                                    ]),
                                    aiot.__ce__("div", {
                                        __vm__: _vm_,
                                        __opts__: {
                                            classList: [
                                                "sprite-box"
                                            ],
                                            events: {
                                                click: function(evt) {
                                                    return _vm_.next(evt);
                                                }
                                            }
                                        }
                                    }, [
                                        aiot.__ce__("image", {
                                            __vm__: _vm_,
                                            __opts__: {
                                                classList: [
                                                    "sprite"
                                                ],
                                                src: function() {
                                                    return _vm_.frame;
                                                }
                                            }
                                        }, [])
                                    ]),
                                    aiot.__ci__({
                                        __vm__: _vm_,
                                        __opts__: {
                                            shown: function() {
                                                return _vm_.bubble.show;
                                            }
                                        }
                                    }, function() {
                                        return [
                                            aiot.__ce__("div", {
                                                __vm__: _vm_,
                                                __opts__: {
                                                    classList: [
                                                        "bubble"
                                                    ],
                                                    style: function() {
                                                        return __webpack_require__.g.$translateStyle$("border-color: " + _vm_.bubble.color);
                                                    },
                                                    events: {
                                                        click: function(evt) {
                                                            return _vm_.next(evt);
                                                        }
                                                    }
                                                }
                                            }, [
                                                aiot.__ce__("div", {
                                                    __vm__: _vm_,
                                                    __opts__: {
                                                        classList: [
                                                            "bubble-row"
                                                        ]
                                                    }
                                                }, [
                                                    aiot.__ce__("text", {
                                                        __vm__: _vm_,
                                                        __opts__: {
                                                            classList: [
                                                                "bubble-project"
                                                            ],
                                                            value: function() {
                                                                return _vm_.bubble.project;
                                                            }
                                                        }
                                                    }, []),
                                                    aiot.__ci__({
                                                        __vm__: _vm_,
                                                        __opts__: {
                                                            shown: function() {
                                                                return _vm_.bubble.count;
                                                            }
                                                        }
                                                    }, function() {
                                                        return [
                                                            aiot.__ce__("text", {
                                                                __vm__: _vm_,
                                                                __opts__: {
                                                                    classList: [
                                                                        "bubble-count"
                                                                    ],
                                                                    value: function() {
                                                                        return _vm_.bubble.count;
                                                                    }
                                                                }
                                                            }, [])
                                                        ];
                                                    })
                                                ]),
                                                aiot.__ce__("text", {
                                                    __vm__: _vm_,
                                                    __opts__: {
                                                        classList: [
                                                            "bubble-title"
                                                        ],
                                                        value: function() {
                                                            return _vm_.bubble.title;
                                                        }
                                                    }
                                                }, [])
                                            ])
                                        ];
                                    }),
                                    aiot.__ci__({
                                        __vm__: _vm_,
                                        __opts__: {
                                            shown: function() {
                                                return !_vm_.bubble.show;
                                            }
                                        }
                                    }, function() {
                                        return [
                                            aiot.__ce__("div", {
                                                __vm__: _vm_,
                                                __opts__: {
                                                    classList: [
                                                        "bubble-empty"
                                                    ]
                                                }
                                            }, [])
                                        ];
                                    }),
                                    aiot.__ce__("div", {
                                        __vm__: _vm_,
                                        __opts__: {
                                            classList: [
                                                "status"
                                            ]
                                        }
                                    }, [
                                        aiot.__ci__({
                                            __vm__: _vm_,
                                            __opts__: {
                                                shown: function() {
                                                    return _vm_.counts.blocked > 0;
                                                }
                                            }
                                        }, function() {
                                            return [
                                                aiot.__ce__("div", {
                                                    __vm__: _vm_,
                                                    __opts__: {
                                                        classList: [
                                                            "dot-group"
                                                        ]
                                                    }
                                                }, [
                                                    aiot.__ce__("div", {
                                                        __vm__: _vm_,
                                                        __opts__: {
                                                            classList: [
                                                                "dot"
                                                            ],
                                                            style: {
                                                                backgroundColor: "#ff9f1c"
                                                            }
                                                        }
                                                    }, []),
                                                    aiot.__ce__("text", {
                                                        __vm__: _vm_,
                                                        __opts__: {
                                                            classList: [
                                                                "dot-n"
                                                            ],
                                                            value: function() {
                                                                return _vm_.counts.blocked;
                                                            }
                                                        }
                                                    }, [])
                                                ])
                                            ];
                                        }),
                                        aiot.__ce__("div", {
                                            __vm__: _vm_,
                                            __opts__: {
                                                classList: [
                                                    "dot-group"
                                                ]
                                            }
                                        }, [
                                            aiot.__ce__("div", {
                                                __vm__: _vm_,
                                                __opts__: {
                                                    classList: [
                                                        "dot"
                                                    ],
                                                    style: {
                                                        backgroundColor: "#2ea0ff"
                                                    }
                                                }
                                            }, []),
                                            aiot.__ce__("text", {
                                                __vm__: _vm_,
                                                __opts__: {
                                                    classList: [
                                                        "dot-n"
                                                    ],
                                                    value: function() {
                                                        return _vm_.counts.working;
                                                    }
                                                }
                                            }, [])
                                        ]),
                                        aiot.__ce__("div", {
                                            __vm__: _vm_,
                                            __opts__: {
                                                classList: [
                                                    "dot-group"
                                                ]
                                            }
                                        }, [
                                            aiot.__ce__("div", {
                                                __vm__: _vm_,
                                                __opts__: {
                                                    classList: [
                                                        "dot"
                                                    ],
                                                    style: {
                                                        backgroundColor: "#3ddc84"
                                                    }
                                                }
                                            }, []),
                                            aiot.__ce__("text", {
                                                __vm__: _vm_,
                                                __opts__: {
                                                    classList: [
                                                        "dot-n"
                                                    ],
                                                    value: function() {
                                                        return _vm_.counts.done;
                                                    }
                                                }
                                            }, [])
                                        ]),
                                        aiot.__ce__("div", {
                                            __vm__: _vm_,
                                            __opts__: {
                                                classList: [
                                                    "dot-group"
                                                ]
                                            }
                                        }, [
                                            aiot.__ce__("div", {
                                                __vm__: _vm_,
                                                __opts__: {
                                                    classList: [
                                                        "dot"
                                                    ],
                                                    style: {
                                                        backgroundColor: "#5c6068"
                                                    }
                                                }
                                            }, []),
                                            aiot.__ce__("text", {
                                                __vm__: _vm_,
                                                __opts__: {
                                                    classList: [
                                                        "dot-n"
                                                    ],
                                                    value: function() {
                                                        return _vm_.counts.idle;
                                                    }
                                                }
                                            }, [])
                                        ]),
                                        aiot.__ce__("div", {
                                            __vm__: _vm_,
                                            __opts__: {
                                                classList: [
                                                    "spacer"
                                                ]
                                            }
                                        }, []),
                                        aiot.__ce__("div", {
                                            __vm__: _vm_,
                                            __opts__: {
                                                classList: [
                                                    "tally"
                                                ],
                                                events: {
                                                    click: function(evt) {
                                                        return _vm_.openToday(evt);
                                                    }
                                                }
                                            }
                                        }, [
                                            aiot.__ci__({
                                                __vm__: _vm_,
                                                __opts__: {
                                                    shown: function() {
                                                        return _vm_.costText;
                                                    }
                                                }
                                            }, function() {
                                                return [
                                                    aiot.__ce__("text", {
                                                        __vm__: _vm_,
                                                        __opts__: {
                                                            classList: [
                                                                "cost"
                                                            ],
                                                            style: function() {
                                                                return __webpack_require__.g.$translateStyle$("color: " + _vm_.costColor);
                                                            },
                                                            value: function() {
                                                                return _vm_.costText;
                                                            }
                                                        }
                                                    }, [])
                                                ];
                                            }),
                                            aiot.__ce__("image", {
                                                __vm__: _vm_,
                                                __opts__: {
                                                    classList: [
                                                        "fish"
                                                    ],
                                                    src: "/common/pet/fish.png"
                                                }
                                            }, []),
                                            aiot.__ce__("text", {
                                                __vm__: _vm_,
                                                __opts__: {
                                                    classList: [
                                                        "fish-n"
                                                    ],
                                                    style: function() {
                                                        return __webpack_require__.g.$translateStyle$("color: " + _vm_.fishColor);
                                                    },
                                                    value: function() {
                                                        return _vm_.fish;
                                                    }
                                                }
                                            }, [])
                                        ])
                                    ])
                                ]),
                                aiot.__ce__("div", {
                                    __vm__: _vm_,
                                    __opts__: {
                                        classList: [
                                            "today"
                                        ],
                                        style: function() {
                                            return __webpack_require__.g.$translateStyle$("height: " + _vm_.pageH + "px");
                                        }
                                    }
                                }, [
                                    aiot.__ce__("div", {
                                        __vm__: _vm_,
                                        __opts__: {
                                            classList: [
                                                "today-head"
                                            ]
                                        }
                                    }, [
                                        aiot.__ce__("text", {
                                            __vm__: _vm_,
                                            __opts__: {
                                                classList: [
                                                    "today-title"
                                                ],
                                                value: "今日"
                                            }
                                        }, []),
                                        aiot.__ce__("div", {
                                            __vm__: _vm_,
                                            __opts__: {
                                                classList: [
                                                    "spacer"
                                                ]
                                            }
                                        }, []),
                                        aiot.__ce__("text", {
                                            __vm__: _vm_,
                                            __opts__: {
                                                classList: [
                                                    "today-date"
                                                ],
                                                value: function() {
                                                    return _vm_.dateText;
                                                }
                                            }
                                        }, [])
                                    ]),
                                    aiot.__ce__("div", {
                                        __vm__: _vm_,
                                        __opts__: {
                                            classList: [
                                                "blocks"
                                            ]
                                        }
                                    }, [
                                        aiot.__ce__("div", {
                                            __vm__: _vm_,
                                            __opts__: {
                                                classList: [
                                                    "block"
                                                ],
                                                events: {
                                                    click: function(evt) {
                                                        return _vm_.pick("done", evt);
                                                    }
                                                }
                                            }
                                        }, [
                                            aiot.__ce__("div", {
                                                __vm__: _vm_,
                                                __opts__: {
                                                    classList: [
                                                        "block-row"
                                                    ]
                                                }
                                            }, [
                                                aiot.__ce__("image", {
                                                    __vm__: _vm_,
                                                    __opts__: {
                                                        classList: [
                                                            "block-fish"
                                                        ],
                                                        src: "/common/pet/fish.png"
                                                    }
                                                }, []),
                                                aiot.__ce__("text", {
                                                    __vm__: _vm_,
                                                    __opts__: {
                                                        classList: [
                                                            "block-num"
                                                        ],
                                                        style: {
                                                            color: "#e29640"
                                                        },
                                                        value: function() {
                                                            return _vm_.fish;
                                                        }
                                                    }
                                                }, [])
                                            ]),
                                            aiot.__ce__("text", {
                                                __vm__: _vm_,
                                                __opts__: {
                                                    classList: [
                                                        "block-label"
                                                    ],
                                                    value: "完成的任务"
                                                }
                                            }, []),
                                            aiot.__ce__("div", {
                                                __vm__: _vm_,
                                                __opts__: {
                                                    classList: [
                                                        "mark"
                                                    ],
                                                    style: function() {
                                                        return __webpack_require__.g.$translateStyle$("background-color: " + ("done" === _vm_.metric ? "#e29640" : "#22252c"));
                                                    }
                                                }
                                            }, [])
                                        ]),
                                        aiot.__ce__("div", {
                                            __vm__: _vm_,
                                            __opts__: {
                                                classList: [
                                                    "spacer"
                                                ]
                                            }
                                        }, []),
                                        aiot.__ce__("div", {
                                            __vm__: _vm_,
                                            __opts__: {
                                                classList: [
                                                    "block"
                                                ],
                                                events: {
                                                    click: function(evt) {
                                                        return _vm_.pick("usd", evt);
                                                    }
                                                }
                                            }
                                        }, [
                                            aiot.__ce__("text", {
                                                __vm__: _vm_,
                                                __opts__: {
                                                    classList: [
                                                        "block-num"
                                                    ],
                                                    style: {
                                                        color: "#6fb6ff"
                                                    },
                                                    value: function() {
                                                        return _vm_.costBig;
                                                    }
                                                }
                                            }, []),
                                            aiot.__ce__("text", {
                                                __vm__: _vm_,
                                                __opts__: {
                                                    classList: [
                                                        "block-label"
                                                    ],
                                                    value: "Claude 花的钱"
                                                }
                                            }, []),
                                            aiot.__ce__("div", {
                                                __vm__: _vm_,
                                                __opts__: {
                                                    classList: [
                                                        "mark"
                                                    ],
                                                    style: function() {
                                                        return __webpack_require__.g.$translateStyle$("background-color: " + ("usd" === _vm_.metric ? "#6fb6ff" : "#22252c"));
                                                    }
                                                }
                                            }, [])
                                        ])
                                    ]),
                                    aiot.__ce__("div", {
                                        __vm__: _vm_,
                                        __opts__: {
                                            classList: [
                                                "chart"
                                            ]
                                        }
                                    }, [
                                        aiot.__cf__({
                                            __vm__: _vm_,
                                            __opts__: {
                                                exp: function() {
                                                    return {
                                                        __list__: _vm_.bars,
                                                        __tid__: "date"
                                                    };
                                                },
                                                key: "$idx",
                                                value: "$item"
                                            }
                                        }, function($idx, $item) {
                                            return [
                                                aiot.__ce__("div", {
                                                    __vm__: _vm_,
                                                    __opts__: {
                                                        classList: [
                                                            "bar-col"
                                                        ]
                                                    }
                                                }, [
                                                    aiot.__ce__("text", {
                                                        __vm__: _vm_,
                                                        __opts__: {
                                                            classList: [
                                                                "bar-label"
                                                            ],
                                                            style: function() {
                                                                return __webpack_require__.g.$translateStyle$("color: " + $item.labelColor);
                                                            },
                                                            value: function() {
                                                                return $item.label;
                                                            }
                                                        }
                                                    }, []),
                                                    aiot.__ce__("div", {
                                                        __vm__: _vm_,
                                                        __opts__: {
                                                            classList: [
                                                                "bar"
                                                            ],
                                                            style: function() {
                                                                return __webpack_require__.g.$translateStyle$("height: " + $item.h + "px; background-color: " + $item.color);
                                                            }
                                                        }
                                                    }, []),
                                                    aiot.__ce__("text", {
                                                        __vm__: _vm_,
                                                        __opts__: {
                                                            classList: [
                                                                "bar-day"
                                                            ],
                                                            style: function() {
                                                                return __webpack_require__.g.$translateStyle$("color: " + $item.dayColor);
                                                            },
                                                            value: function() {
                                                                return $item.weekday;
                                                            }
                                                        }
                                                    }, [])
                                                ])
                                            ];
                                        })
                                    ]),
                                    aiot.__ce__("text", {
                                        __vm__: _vm_,
                                        __opts__: {
                                            classList: [
                                                "section"
                                            ],
                                            value: "额度"
                                        }
                                    }, []),
                                    aiot.__cf__({
                                        __vm__: _vm_,
                                        __opts__: {
                                            exp: function() {
                                                return {
                                                    __list__: _vm_.quotas,
                                                    __tid__: "label"
                                                };
                                            },
                                            key: "$idx",
                                            value: "$item"
                                        }
                                    }, function($idx, $item) {
                                        return [
                                            aiot.__ce__("div", {
                                                __vm__: _vm_,
                                                __opts__: {
                                                    classList: [
                                                        "q-row"
                                                    ]
                                                }
                                            }, [
                                                aiot.__ce__("text", {
                                                    __vm__: _vm_,
                                                    __opts__: {
                                                        classList: [
                                                            "q-label"
                                                        ],
                                                        value: function() {
                                                            return $item.label;
                                                        }
                                                    }
                                                }, []),
                                                aiot.__ce__("div", {
                                                    __vm__: _vm_,
                                                    __opts__: {
                                                        classList: [
                                                            "q-track"
                                                        ]
                                                    }
                                                }, [
                                                    aiot.__ce__("div", {
                                                        __vm__: _vm_,
                                                        __opts__: {
                                                            classList: [
                                                                "q-fill"
                                                            ],
                                                            style: function() {
                                                                return __webpack_require__.g.$translateStyle$("width: " + $item.w + "px; background-color: " + $item.color);
                                                            }
                                                        }
                                                    }, [])
                                                ]),
                                                aiot.__ce__("div", {
                                                    __vm__: _vm_,
                                                    __opts__: {
                                                        classList: [
                                                            "spacer"
                                                        ]
                                                    }
                                                }, []),
                                                aiot.__ce__("text", {
                                                    __vm__: _vm_,
                                                    __opts__: {
                                                        classList: [
                                                            "q-text"
                                                        ],
                                                        value: function() {
                                                            return $item.text;
                                                        }
                                                    }
                                                }, [])
                                            ])
                                        ];
                                    })
                                ])
                            ])
                        ]);
                    };
                    $app_exports$['entry'] = function($app_exports$) {
                        $app_script$({}, $app_exports$, $app_require$1);
                        $app_exports$.default.template = $app_template$;
                        $app_exports$.default.style = $app_style$;
                    };
                })();
            })();
        };
        return createPageHandler();
    })(global, globalThis, window, $app_exports$, $app_evaluate$);
}
