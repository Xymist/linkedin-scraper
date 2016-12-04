// ==UserScript==
// @name Collect LinkedIn Data
// @namespace http://github.com/Xymist
// @version 0.1
// @description Grabs data from LinkedIn profiles
// @match https://www.linkedin.com/in/*
// @include http://www.linkedin.com/*
// @include https://www.linkedin.com/*
// @copyright 2016+ Xymist
// @require http://code.jquery.com/jquery-latest.js
// @grant GM_xmlhttpRequest
// @connect *
// ==/UserScript==

(function() {
    'use strict';

    var pageURLCheckTimer = setInterval(
        function() {
            if (
                this.lastPathStr !== location.pathname ||
                this.lastQueryStr !== location.search ||
                this.lastPathStr === null ||
                this.lastQueryStr === null
            ) {
                this.lastPathStr = location.pathname;
                this.lastQueryStr = location.search;
                sendLeadDetails();
            }
        },
        200
    );

    function sendLeadDetails() {
        var userName = 'HenryRackley';
        var userPass = '';

        var leadDetails = {};

        leadDetails.fullName = $('.full-name').text();
        leadDetails.title = $('.title').text();
        leadDetails.company = $('#overview-summary-current td ol li span strong a').text().split(',')[0];
        leadDetails.email = $('#email-view ul li a').text();
        leadDetails.phone = $('#phone-view ul li').text();
        leadDetails.url = window.location.href;

        var newLead = {};
        newLead.userName = userName;
        newLead.userPass = userPass;
        newLead.leadDetails = leadDetails;

        var req = JSON.stringify(newLead);

        GM_xmlhttpRequest({
            method: 'POST',
            url: 'https://lis.jamieduerden.me/recordlead',
            data: req,
            headers: {
                'Content-Type': 'application/x-www-form-urlencoded',
            },
        });
    }
})();
