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
                var __webpack_modules__ = {};
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
                var $app_style$ = [
                    [
                        [
                            [
                                0,
                                "root"
                            ]
                        ],
                        {
                            width: "212px",
                            flexDirection: "column"
                        }
                    ],
                    [
                        [
                            [
                                1,
                                "root"
                            ]
                        ],
                        {
                            backgroundColor: "#000000"
                        }
                    ],
                    [
                        [
                            [
                                2,
                                "text"
                            ]
                        ],
                        {
                            fontSize: "14px"
                        }
                    ],
                    [
                        [
                            [
                                0,
                                "a"
                            ]
                        ],
                        {
                            marginTop: "2px",
                            marginRight: "2px",
                            marginBottom: "2px",
                            marginLeft: "2px"
                        }
                    ],
                    [
                        [
                            [
                                0,
                                "b"
                            ]
                        ],
                        {
                            marginTop: "2px",
                            marginRight: "2px",
                            marginBottom: "2px",
                            marginLeft: "2px"
                        }
                    ],
                    [
                        [
                            [
                                0,
                                "c"
                            ]
                        ],
                        {
                            marginTop: "1px",
                            marginRight: "2px",
                            marginBottom: "3px",
                            marginLeft: "2px",
                            paddingTop: "4px",
                            paddingRight: "6px",
                            paddingBottom: "4px",
                            paddingLeft: "6px"
                        }
                    ],
                    [
                        [
                            [
                                0,
                                "d"
                            ]
                        ],
                        {
                            paddingTop: "1px",
                            paddingRight: "2px",
                            paddingBottom: "3px",
                            paddingLeft: "9px"
                        }
                    ],
                    [
                        [
                            [
                                0,
                                "e"
                            ]
                        ],
                        {
                            borderTopColor: "#ff0000",
                            borderRightColor: "#ff0000",
                            borderBottomColor: "#ff0000",
                            borderLeftColor: "#ff0000",
                            borderStyle: "solid",
                            borderTopWidth: "1px",
                            borderRightWidth: "1px",
                            borderBottomWidth: "1px",
                            borderLeftWidth: "1px",
                            borderRadius: "4px"
                        }
                    ],
                    [
                        [
                            [
                                0,
                                "f"
                            ]
                        ],
                        {
                            borderTopWidth: "2px",
                            borderRightWidth: "2px",
                            borderBottomWidth: "2px",
                            borderLeftWidth: "3px",
                            borderTopColor: "#00ff00",
                            borderRightColor: "#00ff00",
                            borderBottomColor: "#00ff00",
                            borderLeftColor: "#0000ff",
                            borderLeftStyle: "dashed"
                        }
                    ],
                    [
                        [
                            [
                                0,
                                "g"
                            ]
                        ],
                        {
                            flex: 1,
                            opacity: 0.5,
                            lines: 2,
                            flexGrow: 1,
                            fontWeight: "bold"
                        }
                    ],
                    [
                        [
                            [
                                0,
                                "h"
                            ]
                        ],
                        {
                            width: "50%",
                            color: "#ffffff",
                            backgroundColor: "rgba(0, 0, 0, 0.5)"
                        }
                    ],
                    [
                        [
                            [
                                0,
                                "i"
                            ]
                        ],
                        {
                            color: "red",
                            textOverflow: "ellipsis",
                            justifyContent: "space-between"
                        }
                    ],
                    [
                        [
                            [
                                0,
                                "j"
                            ]
                        ],
                        {
                            left: "-2px",
                            position: "absolute",
                            top: 0
                        }
                    ]
                ];
                var $app_script$ = function __scriptModule__(module, exports, $app_require$1) {
                    "use strict";
                    Object.defineProperty(exports, "__esModule", {
                        value: true
                    });
                    exports.default = void 0;
                    var _default = exports.default = {
                        private: {
                            title: 'hi',
                            a: 1,
                            b: 2
                        },
                        tap () {},
                        pick () {},
                        hold () {},
                        onSwipe () {},
                        go () {},
                        pageChange () {},
                        fmt () {}
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
                            classList: function() {
                                const $classValue$ = "root " + _vm_.rootClass;
                                if ('string' == typeof $classValue$) return $classValue$.split(' ').map((item)=>item.trim()).filter(Boolean);
                                return $classValue$;
                            },
                            id: "root",
                            style: function() {
                                return __webpack_require__.g.$translateStyle$("height: " + _vm_.h + "px");
                            }
                        }
                    }, [
                        aiot.__ce__("text", {
                            __vm__: _vm_,
                            __opts__: {
                                classList: [
                                    "title"
                                ],
                                value: function() {
                                    return _vm_.title;
                                }
                            }
                        }, []),
                        aiot.__ce__("text", {
                            __vm__: _vm_,
                            __opts__: {
                                value: "静态文字"
                            }
                        }, []),
                        aiot.__ce__("text", {
                            __vm__: _vm_,
                            __opts__: {
                                value: function() {
                                    return "前面 " + _vm_.a + " 中间 " + (_vm_.b + 1) + " 后面";
                                }
                            }
                        }, []),
                        aiot.__ce__("text", {
                            __vm__: _vm_,
                            __opts__: {
                                classList: [
                                    "mix"
                                ]
                            }
                        }, [
                            aiot.__ce__("span", {
                                __vm__: _vm_,
                                __opts__: {
                                    value: "共"
                                }
                            }),
                            aiot.__ce__("span", {
                                __vm__: _vm_,
                                __opts__: {
                                    style: {
                                        color: "#ff0000"
                                    },
                                    value: function() {
                                        return _vm_.n;
                                    }
                                }
                            }, []),
                            aiot.__ce__("span", {
                                __vm__: _vm_,
                                __opts__: {
                                    value: "个"
                                }
                            })
                        ]),
                        aiot.__ci__({
                            __vm__: _vm_,
                            __opts__: {
                                shown: function() {
                                    return "a" === _vm_.mode;
                                }
                            }
                        }, function() {
                            return [
                                aiot.__ce__("text", {
                                    __vm__: _vm_,
                                    __opts__: {
                                        value: "A"
                                    }
                                }, [])
                            ];
                        }),
                        aiot.__ci__({
                            __vm__: _vm_,
                            __opts__: {
                                shown: function() {
                                    return "a" !== _vm_.mode && "b" === _vm_.mode;
                                }
                            }
                        }, function() {
                            return [
                                aiot.__ce__("text", {
                                    __vm__: _vm_,
                                    __opts__: {
                                        value: "B"
                                    }
                                }, [])
                            ];
                        }),
                        aiot.__ci__({
                            __vm__: _vm_,
                            __opts__: {
                                shown: function() {
                                    return "b" !== _vm_.mode && "a" !== _vm_.mode && "c" === _vm_.mode;
                                }
                            }
                        }, function() {
                            return [
                                aiot.__ce__("text", {
                                    __vm__: _vm_,
                                    __opts__: {
                                        value: "C"
                                    }
                                }, [])
                            ];
                        }),
                        aiot.__ci__({
                            __vm__: _vm_,
                            __opts__: {
                                shown: function() {
                                    return "c" !== _vm_.mode && "b" !== _vm_.mode && "a" !== _vm_.mode;
                                }
                            }
                        }, function() {
                            return [
                                aiot.__ce__("text", {
                                    __vm__: _vm_,
                                    __opts__: {
                                        value: "other"
                                    }
                                }, [])
                            ];
                        }),
                        aiot.__ci__({
                            __vm__: _vm_,
                            __opts__: {
                                shown: function() {
                                    return _vm_.show;
                                }
                            }
                        }, function() {
                            return [
                                aiot.__ce__("text", {
                                    __vm__: _vm_,
                                    __opts__: {
                                        value: "在 block 里"
                                    }
                                }, []),
                                aiot.__ce__("div", {
                                    __vm__: _vm_,
                                    __opts__: {
                                        classList: [
                                            "box"
                                        ],
                                        style: function() {
                                            return __webpack_require__.g.$translateStyle$("background-color: " + _vm_.color + "; width: " + _vm_.w + "px");
                                        }
                                    }
                                }, [])
                            ];
                        }),
                        aiot.__cf__({
                            __vm__: _vm_,
                            __opts__: {
                                exp: function() {
                                    return _vm_.list;
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
                                            "row"
                                        ]
                                    }
                                }, [
                                    aiot.__ce__("text", {
                                        __vm__: _vm_,
                                        __opts__: {
                                            value: function() {
                                                return $idx + ": " + $item.name;
                                            }
                                        }
                                    }, [])
                                ])
                            ];
                        }),
                        aiot.__cf__({
                            __vm__: _vm_,
                            __opts__: {
                                exp: function() {
                                    return {
                                        __list__: _vm_.list,
                                        __tid__: "id"
                                    };
                                },
                                key: "$idx",
                                value: "item"
                            }
                        }, function($idx, item) {
                            return [
                                aiot.__ce__("div", {
                                    __vm__: _vm_,
                                    __opts__: {}
                                }, [
                                    aiot.__ce__("text", {
                                        __vm__: _vm_,
                                        __opts__: {
                                            value: function() {
                                                return item.name;
                                            }
                                        }
                                    }, [])
                                ])
                            ];
                        }),
                        aiot.__cf__({
                            __vm__: _vm_,
                            __opts__: {
                                exp: function() {
                                    return {
                                        __list__: _vm_.list,
                                        __tid__: "id"
                                    };
                                },
                                key: "i",
                                value: "item"
                            }
                        }, function(i, item) {
                            return [
                                aiot.__ce__("div", {
                                    __vm__: _vm_,
                                    __opts__: {
                                        events: {
                                            click: function(evt) {
                                                return _vm_.pick(item.id, i, evt);
                                            }
                                        }
                                    }
                                }, [
                                    aiot.__ce__("text", {
                                        __vm__: _vm_,
                                        __opts__: {
                                            style: function() {
                                                return __webpack_require__.g.$translateStyle$("color: " + (i === _vm_.sel ? "#ffffff" : "#888888"));
                                            },
                                            value: function() {
                                                return item.name;
                                            }
                                        }
                                    }, []),
                                    aiot.__cf__({
                                        __vm__: _vm_,
                                        __opts__: {
                                            exp: function() {
                                                return item.subs;
                                            },
                                            key: "j",
                                            value: "sub"
                                        }
                                    }, function(j, sub) {
                                        return [
                                            aiot.__ce__("div", {
                                                __vm__: _vm_,
                                                __opts__: {}
                                            }, [
                                                aiot.__ce__("text", {
                                                    __vm__: _vm_,
                                                    __opts__: {
                                                        value: function() {
                                                            return i + "-" + j + " " + sub + " " + _vm_.title;
                                                        }
                                                    }
                                                }, [])
                                            ])
                                        ];
                                    })
                                ])
                            ];
                        }),
                        aiot.__ce__("div", {
                            __vm__: _vm_,
                            __opts__: {
                                events: {
                                    click: function(evt) {
                                        return _vm_.tap(evt);
                                    },
                                    longpress: function(evt) {
                                        return _vm_.hold("x", evt);
                                    },
                                    swipe: (e)=>_vm_.onSwipe(e.direction)
                                }
                            }
                        }, []),
                        aiot.__ce__("div", {
                            __vm__: _vm_,
                            __opts__: {
                                events: {
                                    click: function(evt) {
                                        return _vm_.go(evt, evt);
                                    }
                                }
                            }
                        }, []),
                        aiot.__ce__("image", {
                            __vm__: _vm_,
                            __opts__: {
                                src: "/assets/a.png",
                                classList: [
                                    "img"
                                ]
                            }
                        }, []),
                        aiot.__ce__("image", {
                            __vm__: _vm_,
                            __opts__: {
                                src: function() {
                                    return "/assets/" + _vm_.name + ".png";
                                }
                            }
                        }, []),
                        aiot.__ce__("image", {
                            __vm__: _vm_,
                            __opts__: {
                                src: function() {
                                    return "/assets/" + _vm_.name + ".png";
                                }
                            }
                        }, []),
                        aiot.__ce__("text", {
                            __vm__: _vm_,
                            __opts__: {
                                style: {
                                    width: "100px",
                                    paddingTop: "2px",
                                    paddingRight: "4px",
                                    paddingBottom: "2px",
                                    paddingLeft: "4px",
                                    color: "#ffffff",
                                    opacity: 0.5
                                },
                                value: "静态样式"
                            }
                        }, []),
                        aiot.__ce__("text", {
                            __vm__: _vm_,
                            __opts__: {
                                classList: function() {
                                    const $classValue$ = _vm_.active ? "on" : "off";
                                    if ('string' == typeof $classValue$) return $classValue$.split(' ').map((item)=>item.trim()).filter(Boolean);
                                    return $classValue$;
                                },
                                value: function() {
                                    return Math.max(_vm_.a, _vm_.b) + " " + _vm_.obj[_vm_.key] + " " + _vm_.fmt(_vm_.a, "x") + " " + (_vm_.list.length > 0 ? _vm_.list[0].name : "");
                                }
                            }
                        }, []),
                        aiot.__ce__("text", {
                            __vm__: _vm_,
                            __opts__: {
                                value: function() {
                                    return ({
                                        x: _vm_.a
                                    }).x;
                                }
                            }
                        }, []),
                        aiot.__ce__("div", {
                            __vm__: _vm_,
                            __opts__: {
                                dataset: {
                                    id: function() {
                                        return _vm_.a;
                                    },
                                    longName: "static"
                                }
                            }
                        }, []),
                        aiot.__ce__("swiper", {
                            __vm__: _vm_,
                            __opts__: {
                                vertical: "true",
                                index: function() {
                                    return _vm_.page;
                                },
                                events: {
                                    change: function(evt) {
                                        return _vm_.pageChange(evt);
                                    }
                                }
                            }
                        }, [
                            aiot.__ce__("div", {
                                __vm__: _vm_,
                                __opts__: {}
                            }, [])
                        ]),
                        aiot.__ce__("input", {
                            __vm__: _vm_,
                            __opts__: {
                                type: "button",
                                value: function() {
                                    return _vm_.label;
                                },
                                disabled: "true"
                            }
                        }, []),
                        aiot.__ce__("progress", {
                            __vm__: _vm_,
                            __opts__: {
                                percent: function() {
                                    return _vm_.pct;
                                }
                            }
                        }, []),
                        aiot.__ce__("div", {
                            __vm__: _vm_,
                            __opts__: {
                                show: function() {
                                    return _vm_.visible;
                                }
                            }
                        }, [])
                    ]);
                };
                $app_exports$['entry'] = function($app_exports$) {
                    $app_script$({}, $app_exports$, $app_require$1);
                    $app_exports$.default.template = $app_template$;
                    $app_exports$.default.style = $app_style$;
                };
            })();
        };
        return createPageHandler();
    })(global, globalThis, window, $app_exports$, $app_evaluate$);
}
