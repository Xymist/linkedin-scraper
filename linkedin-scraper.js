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

(function () {
    'use strict';

    var pageURLCheckTimer = setInterval(
        function () {
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

        var fullName = $('.full-name').text().split(' ');

        leadDetails.firstName = fullName[0];
        leadDetails.lastName = fullName[fullName.length - 1];
        leadDetails.title = $('.title').text();
        leadDetails.company = $('#overview-summary-current td ol li span strong a').text();
        leadDetails.email = $('#email-view ul li a').text();
        leadDetails.phone = $('#phone-view ul li').text();
        leadDetails.url = window.location.href;

        var newLead = {};
        newLead.userName = userName;
        newLead.userPass = userPass;
        newLead.leadDetails = leadDetails;

        var req = JSON.stringify(newLead);

        GM_xmlhttpRequest({
            method: "POST",
            url: "https://lis.jamieduerden.me/recordlead",
            data: req,
            headers: {
                "Content-Type": "application/x-www-form-urlencoded"
            }
        });
    }
})();
