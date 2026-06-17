// Copyright 2026 Inngest Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

test('updateHostnames preserves manual params when selected target is unchanged', () => {
    const harness = loadLocalJS();
    const mac = harness.elements.mac;
    const target = harness.elements.target;
    const params = harness.elements.params;

    mac.options = [{
        value: 'aa:bb:cc:dd:ee:ff',
        text: 'aa:bb:cc:dd:ee:ff - 192.0.2.10',
        data: {}
    }];
    mac.value = 'aa:bb:cc:dd:ee:ff';
    target.options = [{
        value: '',
        text: 'Select an iPXE script',
        data: {}
    }, {
        value: 'debian12',
        text: 'Debian 12',
        data: {
            script: 'debian.ipxe',
            env: 'prod'
        }
    }];
    target.value = 'debian12';
    params.children = ['typed-hostname=value'];
    harness.context.systemsByMac = {
        'aa:bb:cc:dd:ee:ff': {
            AllowedTargets: [{
                Name: 'debian12',
                Label: 'Debian 12',
                Script: 'debian.ipxe',
                Environment: 'prod'
            }]
        }
    };
    harness.serverResponses.push([{
        Mac: 'aa:bb:cc:dd:ee:ff',
        IP: '192.0.2.10',
        Hostname: '',
        AllowedTargets: [{
            Name: 'debian12',
            Label: 'Debian 12',
            Script: 'debian.ipxe',
            Environment: 'prod'
        }]
    }]);

    harness.context.updateHostnames();

    assert.deepEqual(params.children, ['typed-hostname=value']);
    assert.equal(harness.scriptParamRequests, 0);
});

function loadLocalJS() {
    const elements = {
        mac: selectElement('mac'),
        target: selectElement('target'),
        params: {children: []},
        systems: {},
        loading: {}
    };
    const serverResponses = [];
    const harness = {
        elements,
        serverResponses,
        scriptParamRequests: 0
    };

    function $(selector) {
        if (selector === context.document) {
            return {ready: () => {}};
        }
        if (typeof selector === 'string' && selector === '<option>') {
            return optionBuilder();
        }
        if (selector && selector.__optionBuilder) {
            return selector;
        }
        if (selector && typeof selector.data === 'function') {
            return selector;
        }
        if (selector && selector.__option) {
            return {
                text() {
                    return selector.text;
                }
            };
        }
        switch (selector) {
        case '#mac':
        case 'select[name="mac"]':
            return collection(elements.mac);
        case '#mac option':
            return optionsCollection(elements.mac);
        case '#target':
        case 'select[name="target"]':
            return collection(elements.target);
        case '.params-container':
            return collection(elements.params);
        case '#systems':
            return collection(elements.systems);
        case '#loading':
            return collection(elements.loading);
        default:
            throw new Error(`unexpected selector ${selector}`);
        }
    }

    $.each = function (items, callback) {
        for (const item of items) {
            callback.call(item);
        }
    };
    $.getJSON = function (url, callback) {
        assert.equal(url, '/ajax/servers');
        callback(serverResponses.shift());
    };
    $.get = function () {
        harness.scriptParamRequests += 1;
        throw new Error('script params should not be reloaded when target is unchanged');
    };

    const context = {
        $,
        console,
        document: {},
        window: {
            setInterval: () => {},
            setTimeout: () => {}
        }
    };
    vm.createContext(context);
    vm.runInContext(fs.readFileSync(path.join(__dirname, 'local.js'), 'utf8'), context);
    harness.context = context;
    return harness;
}

function selectElement(name) {
    return {
        name,
        options: [],
        value: ''
    };
}

function optionBuilder() {
    const option = {
        __optionBuilder: true,
        value: '',
        textValue: '',
        dataValues: {},
        attr(name, value) {
            if (name === 'value') {
                this.value = value;
            }
            if (name === 'data-script') {
                this.dataValues.script = value;
            }
            if (name === 'data-env') {
                this.dataValues.env = value;
            }
            return this;
        },
        text(value) {
            if (arguments.length === 0) {
                return this.textValue;
            }
            this.textValue = value;
            return this;
        },
        data(name) {
            return this.dataValues[name];
        }
    };
    return option;
}

function collection(element) {
    return {
        empty() {
            if (element.options) {
                element.options = [];
                element.value = '';
            } else {
                element.children = [];
            }
            return this;
        },
        append(child) {
            if (element.options) {
                appendOption(element, child);
            } else if (element.children) {
                element.children.push(child);
            }
            return this;
        },
        val(value) {
            if (arguments.length === 0) {
                return element.value;
            }
            element.value = element.options.some((option) => option.value === value) ? value : null;
            return this;
        },
        find(selector) {
            assert.equal(selector, 'option:selected');
            const selected = element.options.find((option) => option.value === element.value);
            return {
                text() {
                    return selected && selected.text;
                },
                data(name) {
                    return selected && selected.data[name];
                }
            };
        },
        hide() {
            return this;
        },
        fadeIn() {
            return this;
        },
        fadeOut() {
            return this;
        },
        removeClass() {
            return this;
        }
    };
}

function optionsCollection(element) {
    return {
        filter(callback) {
            const matches = element.options.filter((option) => callback.call({
                __option: true,
                text: option.text
            }));
            return {
                prop(name, value) {
                    assert.equal(name, 'selected');
                    if (value && matches.length > 0) {
                        element.value = matches[0].value;
                    }
                    return this;
                }
            };
        }
    };
}

function appendOption(element, child) {
    if (typeof child === 'string') {
        const value = child.match(/value="([^"]+)"/)[1];
        const text = child.match(/>([^<]+)<\/option>/)[1];
        element.options.push({value, text, data: {}});
        if (!element.value) {
            element.value = value;
        }
        return;
    }

    element.options.push({
        value: child.value,
        text: child.textValue,
        data: child.dataValues
    });
    if (!element.value && child.value !== '') {
        element.value = child.value;
    }
}
