// Monkey-patch jQuery.
(function() {
	var re = /([^&=]+)=?([^&]*)/g;
	var decodeRE = /\+/g;  // Regex for replacing addition symbol with a space
	var decode = function (str) {return decodeURIComponent( str.replace(decodeRE, " ") );};
	$.parseParams = function(query) {
		var params = {}, e;
		while ( e = re.exec(query) ) {
			var k = decode( e[1] ), v = decode( e[2] );
			if (k.substring(k.length - 2) === '[]') {
				k = k.substring(0, k.length - 2);
				(params[k] || (params[k] = [])).push(v);
			}
			else params[k] = v;
		}
		return params;
	};

    // from https://codepen.io/filippoq/pen/QwogWz/
    $.fn.bmdIframe = function() {
        this.on('hidden.bs.modal', function(){
          $(this).find('iframe').html("").attr("src", "");
        });

        return this;
    };
})();

// From https://stackoverflow.com/questions/10420352/converting-file-size-in-bytes-to-human-readable-string
function humanFileSize(bytes, si) {
    var thresh = si ? 1000 : 1024;
    if(Math.abs(bytes) < thresh) {
        return bytes + ' B';
    }
    var units = si
        ? ['KB','MB','GB','TB','PB','EB','ZB','YB']
        : ['KiB','MiB','GiB','TiB','PiB','EiB','ZiB','YiB'];
    var u = -1;
    do {
        bytes /= thresh;
        ++u;
    } while(Math.abs(bytes) >= thresh && u < units.length - 1);
    return bytes.toFixed(1)+' '+units[u];
}

// Define API request function
function apiReq(type, data, cb) {
	$.ajax({
		type: "POST",
		data: JSON.stringify({
			"q": {
				"type": type,
				"data": data
			}
		}),
		dataType: "json",
		url: "/movies",
		success: function(data) {
			console.log(type + " response:", data);
			cb(data.v);
		},
		error: function(err) {
			console.error(err);
		}
	});
}

// Define scroll disabling and enabling functions.
// left: 37, up: 38, right: 39, down: 40,
// spacebar: 32, pageup: 33, pagedown: 34, end: 35, home: 36

function preventDefault(e) {
  e = e || window.event;
  if (e.preventDefault)
      e.preventDefault();
  e.returnValue = false;
}

function preventDefaultForScrollKeys(e) {
	var keys = {37: 1, 38: 1, 39: 1, 40: 1};
    if (keys[e.keyCode]) {
        preventDefault(e);
        return false;
    }
}

function disableScroll() {
  if (window.addEventListener) // older FF
      window.addEventListener('DOMMouseScroll', preventDefault, false);
  window.onwheel = preventDefault; // modern standard
  window.onmousewheel = document.onmousewheel = preventDefault; // older browsers, IE
  window.ontouchmove  = preventDefault; // mobile
  document.onkeydown  = preventDefaultForScrollKeys;
}

function enableScroll() {
    if (window.removeEventListener)
        window.removeEventListener('DOMMouseScroll', preventDefault, false);
    window.onmousewheel = document.onmousewheel = null;
    window.onwheel = null;
    window.ontouchmove = null;
    document.onkeydown = null;
}

// API wrapper functions.
function getRecommendedMovies(limit, extension) {
	extension = extension || -1;
	return new Promise((resolve, reject) => {
		apiReq("getRecommendedMovies", {
			"extended": extension.toString()
		}, function(data) {
			resolve(data.recommendations);
		});
	});
}

function getWatchlist() {
	return new Promise((resolve, reject) => {
		apiReq("getWatchlist", {
		}, function(data) {
			resolve(data.watchlist);
		});
	});
}

function getWatched() {
	return new Promise((resolve, reject) => {
		apiReq("getHistory", {
		}, function(data) {
			if(data && data.watched){
				resolve(data.watched);
			} else {
				swal({
					title: "Trakt Token",
					text: "Trakt API token is outdated.",
					icon: "warning",
					timer: 2500,
					buttons: false
				});
				resolve([]);
			}
		});
	});
}

function getScrobbles() {
	return new Promise((resolve, reject) => {
		apiReq("getScrobbles", {
		}, function(data) {
			if(data && data.watched){
				resolve(data.watched);
			} else {
				resolve([]);
			}
		});
	});
}

function startAirplay(info) {
	return new Promise((resolve, reject) => {
		apiReq("startAirplayPlayback", {
			"url": info.url,
			"progress": info.progress
		}, function(data) {
			resolve(data);
		});
	});
}

function stopAirplay() {
	return new Promise((resolve, reject) => {
		apiReq("stopAirplayPlayback", {
		}, function(data) {
			resolve(data);
		});
	});
}

function addToHistory(item_type, item_id) {
	return new Promise((resolve, reject) => {
		apiReq("addHistory", {
			"item_type": item_type,
			"item_id": item_id
		}, function(data) {
			resolve(data);
		});
	});
}

function addToWatchlist(item_type, item_id) {
	return new Promise((resolve, reject) => {
		apiReq("addToWatchlist", {
			"item_type": item_type,
			"item_id": item_id
		}, function(data) {
			resolve(data);
		});
	});
}

function searchForItem(keyword) {
	return new Promise((resolve, reject) => {
		apiReq("searchForItem", {
			"keyword": keyword
		}, function(data) {
			resolve(data.results);
		});
	});
}

function lookupItem(imdb_id) {
	return new Promise((resolve, reject) => {
		apiReq("itemLookup", {
			"id": imdb_id
		}, function(data) {
			resolve(data);
		});
	});
}

function resolveItem(imdb_id) {
	return new Promise((resolve, reject) => {
		if(!imdb_id){
			return resolve(null);
		}
		apiReq("imdbIdLookup", {
			"id": imdb_id
		}, function(data) {
			resolve(data);
		});
	});
}

function resolveParallel(ids) {
	return new Promise((resolve, reject) => {
		apiReq("resolveParallel", {
			"ids": ids
		}, function(data) {
			resolve(data);
		});
	});
}

function fetchItem(info) {
	return new Promise((resolve, reject) => {
		apiReq("fetchUri", {
			"uri": info.uri,
			"imdb_id": info.imdb_code
		}, function(data) {
			resolve(data);
		});
	});
}

function associateItem(info) {
	return new Promise((resolve, reject) => {
		apiReq("associateDownload", {
			"cloud_id": info.cloud_id.toString(),
			"imdb_id": info.imdb_code
		}, function(data) {
			resolve(data);
		});
	});
}

function addToCollection(info) {
	return new Promise((resolve, reject) => {
		apiReq("addToCollection", {
			"cloud_id": info.cloud_id.toString(),
			"collection_id": info.collection_id
		}, function(data) {
			resolve(data);
		});
	});
}

function downloadItemInBackground(info) {
	return new Promise((resolve, reject) => {
		apiReq("startBackgroundDownload", {
			"id": info.cloud_id.toString(),
			"uri": info.uri,
			"filename": info.filename
		}, function(data) {
			resolve(data);
		});
	});
}

function updateWatchStatus(imdb_id, progress, state) {
	return new Promise((resolve, reject) => {
		apiReq("updateScrobble", {
			"imdb_code": imdb_id,
			"progress": progress,
			"state": state
		}, function(data) {
			resolve(data);
		});
	});
}

function evictLocalItem(cloud_id) {
	return new Promise((resolve, reject) => {
		apiReq("evictLocalItem", {
			"id": cloud_id
		}, function(data) {
			resolve(data);
		});
	});
}

function intelligentRenameItem(info) {
	return new Promise((resolve, reject) => {
		apiReq("intelligentRenameItem", {
			"id": info.id,
			"title": info.title
		}, function(data) {
			resolve(data);
		});
	});
}

function iCloudStreamUrl(cloud_id) {
	return new Promise((resolve, reject) => {
		apiReq("getiCloudStreamUrl", {
			"id": cloud_id
		}, function(data) {
			resolve(data);
		});
	});
}

function getAssociatedDownloads() {
	return new Promise((resolve, reject) => {
		apiReq("getAssociatedDownloads", {}, function(data) {
			resolve(data);
		});
	});
}

function getDownloads() {
	return new Promise((resolve, reject) => {
		apiReq("getDownloads", {
			//
		}, function(data) {
			data.downloads = data.downloads || [];
			var idsToResolve = data.downloads.map(x => x.imdb_id).filter(x => x && x.length > 0);
			if(idsToResolve.length == 0){
				return resolve(data);
			}
			resolveParallel(idsToResolve).then((resolved) => {
				resolved = resolved.resolved;
				var ret = data.downloads;
				for(var item of resolved) {
					var on = item.imdb_code;
					for (var i = ret.length - 1; i >= 0; i--) {
						if(ret[i].imdb_id === on){
							ret[i].resolved = item;
							// no break statement since multiple downloads with same IMDb ID are possible
						}
					}
				}
				resolve({
					downloads: ret,
					airplay_info: data.airplay_info || {}
				});
			});
		});
	});
}

function getCollections() {
	return new Promise((resolve, reject) => {
		apiReq("getCollections", {}, function(data) {
			resolve(data.collections);
		});
	});
}

// Run on page load.
let player_windows = [];
let win_id = 0;
let child, currentItem, lastDownloadedItem, history, assocDownloads
$(function () {
	// Set up notification permissions
	if(Notification.permission !== "denied" && Notification.permission !== "granted") {
		Notification.requestPermission(function (permission) {
			console.log("New notification permissions:", permission);
		});
	}

	// Function to retrieve HTML markup for a specified rating, scaled 0 to 100.
	var retrieveRatingMarkup = function(rating_percentage){
		var ret_div = $('<div class="star-ratings-css"></div>');
		ret_div.append($('<div class="star-ratings-css-top" style="width: ' + Math.round(rating_percentage) + '%"><span>★</span><span>★</span><span>★</span><span>★</span><span>★</span></div>'));
		ret_div.append($('<div class="star-ratings-css-bottom"><span>★</span><span>★</span><span>★</span><span>★</span><span>★</span></div>'));
		return ret_div.wrap('<p/>').parent().html();
	};

	// Define history functions.
	history = {
		"watchlist": [],
		"watched": []
	};
	var refreshHistoryWatchlist = function() {
		return new Promise((resolve, reject) => {
			getWatchlist().then((data) => {
				history.watchlist = data;
				resolve();
			});
		});
	};
	var refreshHistoryWatched = function() {
		return new Promise((resolve, reject) => {
			getWatched().then((data) => {
				history.watched = data;
				resolve();
			});
		});
	};
	var refreshAssocDownloads = function() {
		return new Promise((resolve, reject) => {
			getAssociatedDownloads().then((data) => {
				assocDownloads = data.downloads;
				resolve();
			});
		});
	};
	var isMovieInWatchlist = function(imdb_code) {
		return history.watchlist.indexOf(imdb_code) !== -1;
	};
	var isMovieInWatched = function(imdb_code) {
		return history.watched.indexOf(imdb_code) !== -1;
	};
	var isMovieInDownloads = function(imdb_code) {
		return assocDownloads.indexOf(imdb_code) !== -1;
	};

	// Function to generate markup for movie poster item in grid.
	var retrieveCoverMarkup = function(on) {
		var li = $('<li class="grid-item"></li>');
		var new_hash = '#watch?' + $.param({
			"title": on.title,
			"id": on.imdb_code
		});
		var watched_hash = "#watched_recommendation?" + $.param({
			"id": on.imdb_code
		});
		var watchlist_hash = "#add_to_watchlist?" + $.param({
			"id": on.imdb_code
		});
		var associate_hash = '#associate_imdb?' + $.param({
			"id": on.imdb_code
		});
		var anc = $('<a class="grid-cell" href="' + new_hash + '"></a>');
		anc.append('<img class="grid-img" src="' + on.cover_image + '" />');
		anc.append('<span class="grid-overlay"></span>');
		var desc = [on.mpaa_rating || "NR"];
		var sort_order = {
			"3D": 1,
			"HD": 2,
			"1080p": 3,
			"720p": 4
		};
		var qualities = on.sources.map((x) => x.quality).sort((a, b) => {
			var x = sort_order[a], y = sort_order[b];
			return (y - x);
		});
		qualities = qualities.filter((x, i) => qualities.indexOf(x) === i); // remove duplicates
		desc = on.title + " <br /> " + desc + "<br />" + qualities.join(" | ");
		var grid_span = $('<span class="grid-text"></span>');
		grid_span.html(desc);
		anc.append(grid_span);
		if(isMovieInWatchlist(on.imdb_code)){
			anc.append($('<a class="bottom-right-corner btn btn-warning"><span class="glyphicon glyphicon-th-list"></span></a>'));
		}
		if(isMovieInWatched(on.imdb_code)){
			anc.append($('<a class="top-right-corner btn btn-success"><span class="glyphicon glyphicon-check"></span></a>'));
		} else {
			anc.append($('<span class="grid-button-bottom-right"><a href="' + watchlist_hash + '" class="btn btn-warning"><span class="glyphicon glyphicon-th-list"></span></a></span>'));
		}
		anc.append($('<span class="grid-button-bottom-left"><a href="' + watched_hash + '" class="btn btn-success"><span class="glyphicon glyphicon-check"></span></a></span>'));
		if(isMovieInDownloads(on.imdb_code)) {
			anc.append($('<a class="top-left-corner-badge btn btn-info"><span class="glyphicon glyphicon-floppy-saved"></span></a>'));
		}
		anc.append($('<span class="top-left-corner"><a href="' + associate_hash + '" class="btn btn-info"><span class="glyphicon glyphicon-random"></span></a></span>'));
		li.append(anc);
		return li;
	};
	var retrieveBlankCoverMarkup = function() {
		var li = $('<li class="grid-item grid-item-blank"></li>');
		var anc = $('<a class="grid-cell" href="#"></a>');
		anc.append('<img class="grid-img" />');
		anc.append('<span class="grid-overlay"></span>');
		anc.append('<span class="grid-text">(Divider)</span>');
		li.append(anc);
		return li;
	};

	var resizeAllGridItems = function(){
		// console.log("Handling resize...");
		var grid = document.getElementById("grid");
		var allItems = Array.from(document.getElementsByClassName("grid-item"));
		if(allItems
			.filter(x => Array.from(document.getElementsByClassName("grid-item-blank")).indexOf(x) === -1)
			.filter(x => $(x).find("img").attr('src') !== "N/A" && ($(x).find("img").attr('src') || "").length > 0)
			.map(x => x.clientHeight)
			.filter(x => x == 0)
			.length > 0
		){
			return setTimeout(resizeAllGridItems, 300);
		}
		var rowNumElements = Math.round($(window).width() / $(allItems[0]).width());
		for(var i = 0; i < allItems.length; i += rowNumElements){
			var slice = allItems.slice(i, i + rowNumElements);
			var heights = slice
				.filter(x => $(x).find("img").attr('src') !== "N/A" && ($(x).find("img").attr('src') || "").length > 0)
				.map(x => x.offsetHeight)
				.filter(x => x > 250);
			// console.log(heights);
			var minHeight = Math.min(...heights);
			// console.log(minHeight);
			for(var j = 0; j < slice.length; j++){
				$(slice[j].querySelector(".grid-img")).height(minHeight);
			}
		}
		$('.loader').hide();
	};
	var resizeWhenLoaded = function() {
		resizeAllGridItems();
	};
	window.addEventListener("resize", resizeAllGridItems);
	/*
	apiReq("resolveParallel", {
		"ids": ["tt0108052", "tt1408101", "tt0075860", "tt0120815", "tt0264464"]
	}, console.log);
	*/

	// Handle navbar clicks.
	$('.nav_btn').click(function(e) {
		$(this).blur();
	});
	$('.nav_btn').hover(function(e) {
		$(this).css('color', 'black');
	}, function(e) {
		$(this).css('color', '');
	});

	// Populate grid with top movies by default, or requested movies if search term exists.
	var onHomepage = false, autoPopulationCounter = 0;
	var customRefreshTitle, customRefreshMessage, customRefreshTimer = 2000;
	var populateGrid = (callback, limit, doneRefreshingDownloads) => {
		if(!doneRefreshingDownloads){
			refreshAssocDownloads().then(() => {
				populateGrid(callback, limit, /*doneRefreshingDownloads=*/true);
			});
			return;
		}
		onHomepage = (callback === getRecommendedMovies); // for auto-population
		autoPopulationCounter = 0; // reset auto-population counter
		$('.loader').show();
		callback(limit).then((data) => {
			console.log(data);
			data = data.filter((x) => !x.unreleased);

			// Initialize the carousel.
			if(!$('#highlights').length){
				var carousel_div = $('<div id="highlights" class="carousel slide" data-ride="carousel"></div>');
				carousel_div.append($('<ol class="carousel-indicators"></ol>'));
				carousel_div.append($('<div class="carousel-inner"></div>'));
				carousel_div.append($('<a class="left carousel-control" href="#highlights" data-slide="prev"><span class="glyphicon glyphicon-chevron-left"></span></a>'));
				carousel_div.append($('<a class="right carousel-control" href="#highlights" data-slide="next"><span class="glyphicon glyphicon-chevron-right"></span></a>'));
				$('#carousel_space').append(carousel_div);
			}

			// Populate the highlight carousel.
			const HIGHLIGHTED_COUNT = 5;
			var highlighted = [];
			$('#highlights').css('opacity', '0');
			$('.carousel-indicators').empty();
			$('.carousel-inner').empty();
			for(var i = 0; i < Math.min(HIGHLIGHTED_COUNT, data.length); i++){
				if(highlighted.length == data.length) break;

				// Add the indicator.
				var li = $('<li data-target="#highlights" data-slide-to="' + i + '" class="active"></li>');
				if(i > 0) li.removeClass("active");
				$('.carousel-indicators').append(li);

				// Add the image.
				var ind = -1;
				var checked = [];
				while(true){
					if(highlighted.length == data.length) break;
					if(checked.length == data.length){
						ind = -1;
						break;
					}
					ind = Math.min(Math.round(Math.random() * data.length), data.length - 1);
					if(checked.indexOf(ind) === -1){
						checked.push(ind);
					}
					if(data[ind].unreleased) continue;
					if(highlighted.indexOf(ind) === -1){
						highlighted.push(ind); // if not used, has already been watched
					} else continue;
					break;
				}
				if(ind == -1) break;
				var cur = data[ind];
				//console.log(cur, highlighted);
				var img_div = $('<div class="item active"></div>');
				if(i > 0) img_div.removeClass("active");
				img_div.append($('<img class="carousel_img" src="' + cur.cover_image + '" alt="' + cur.title + '" />'));
				var cap = $('<div class="carousel-caption"></div>');
				cap.append($('<div class="carousel-title"><h3 style="font-size: 2vw; display: inline;">' + cur.title + '</h3>&nbsp;&nbsp;&nbsp;<p class="rating-box">' + (cur.mpaa_rating || "NR") + '</p></div>'));
				cap.append($(retrieveRatingMarkup(cur.imdb_rating * 10.0)));
				var summary = cur.summary;
				if(!summary || !summary.length) summary = "(no description available)";
				cap.append($('<p class="carousel-summary">' + summary + '</p>'));
				var new_hash = '#watch?' + $.param({
					"title": cur.title,
					"id": cur.imdb_code
				});
				cap.append($('<a href="' + new_hash + '" class="btn btn-success">Watch <span class="glyphicon glyphicon-film"></span></a><br /><br />'));
				img_div.append(cap);
				$('.carousel-inner').append(img_div);
			}
			$('#highlights').css('opacity', '1');

			// Populate the grid.
			$('#grid').empty();
			$('#empty-search').hide();
			if(data.length == 0){
				let notif = new Notification('Search Complete', {
					body: "No results found.",
					silent: true
				});
				notif.onclick = () => {};
				$('#empty-search').show();
				$('#highlights').css('opacity', '0');
			}
			for(var on of data){
				var li = retrieveCoverMarkup(on);
				$('#grid').append(li);
			}
			resizeWhenLoaded();

			// Inform user of success
			if(customRefreshTitle || customRefreshMessage){
				swal({
					title: (customRefreshTitle || "Refreshed Content"),
					text: (customRefreshMessage || "Reloaded items from server."),
					icon: "success",
					buttons: false,
					timer: customRefreshTimer
				});
			}
			customRefreshTitle = customRefreshMessage = undefined;
			customRefreshTimer = 2000;
		});
	};
	var refreshHomepage = function() {
		populateGrid(getRecommendedMovies, /*limit=*/12 * 1);
	};
	$('.loader').show();
	refreshHistoryWatchlist().then(() => {
		refreshHistoryWatched().then(() => {
			$('.loader').hide();
			// customRefreshTitle = "Initialized catalog";
			// customRefreshMessage = "Successfully initialized server catalog.";
			refreshHomepage();
			$(document.body).removeClass("loading");
		});
	});

	// Detect when user has hit bottom of scrollable view and populate with new movies.
	document.addEventListener('scroll', function (event) {
		//console.log($(window).scrollTop() + $(window).height(), $(document).height());
		if($(document).height() == $(window).scrollTop() + $(window).height()) {
			if(onHomepage){
				++autoPopulationCounter;
				console.log("Hit rock bottom.", autoPopulationCounter);
				$('.loader').show();
				disableScroll();
				getRecommendedMovies(12, autoPopulationCounter).then((data) => {
					console.log(data);

					// Append to grid.
					$('#grid').append(retrieveBlankCoverMarkup());
					for(var on of data){
						var li = retrieveCoverMarkup(on);
						$('#grid').append(li);
					}
					resizeWhenLoaded();
					enableScroll();

					/*
					// Inform user.
					swal({
						title: "Extended results",
						text: "Added " + data.length + " " + (data.length == 1 ? "result" : "results") + ".",
						icon: "success",
						buttons: false,
						timer: 1000
					});
					*/
				});
			}
		}
	});

	// Set up iframe.
	$('#frameModal').bmdIframe();
	var openPage = function(opts) {
		opts = $.extend({
			"path": "about:blank",
			"allowFullScreen": false,
			"height": 640,
			"width": 360
		}, opts);

		var iframe = $('#frameModal').find("iframe");
		iframe.attr("src", opts.path);
		/*
		iframe.css({
			"height": opts.height,
			"width": opts.width
		});
		*/

		if(opts.allowFullScreen) iframe.attr("allowfullscreen", "");

		$('#frameModal').modal('show');
	};
	var closePage = function() {
		$('#frameModal').modal('hide');
	}
	var sendFrameMessage = function(obj) {
		var json_str = JSON.stringify(obj);
		var contentWin = $('#frameModal').find("iframe").get(0).contentWindow;
		contentWin.postMessage(json_str, "*");
	};
	var retrieveFileObj = function(folder_id) {
		return new Promise((resolve, reject) => {
			apiReq("oauthApiCall", {
				"path": "folder/" + folder_id,
				"method": "GET"
			}, function(data) {
				console.log("retrieved folder:", data);
				var files = data.files.filter((x) => x.play_video || parseFloat(x.video_progress) >= 0.00)
				files.sort((a, b) => a.size < b.size); // descending order sort by size
				console.log(files);
				var file = files[0];
				resolve(file);
			});
		});
	};
	var retrieveFileUrl = function(folder_id, should_download) {
		return new Promise((resolve, reject) => {
			retrieveFileObj(folder_id).then((file) => {
				apiReq("oauthQuery", {
					"function": (should_download ? "fetch_file" : "fetch_file_view"),
					"data": {
						"folder_file_id": file.folder_file_id.toString()
					}
				}, function(file_data) {
					resolve(file_data);
				});
			});
		});
	}

	// Set up stop AirPlay button.
	$('#airplayStopBtn').click(function(e) {
		e.preventDefault();
		stopAirplay().then((info) => {
			if(!info || !info.result){
				swal({
					title: "Unable to stop playback",
					icon: "error"
				});
			} else {
				swal({
					title: "AirPlay playback stopped",
					text: "Successfully stopped playback.",
					icon: "success",
					buttons: false,
					timer: 2000
				}).then(() => {
					window.history.pushState(null, null, '#view_downloads');
					$(window).trigger('hashchange');
				});
			}
		});
	});

	// Intercept hashchange event and display player.
	$(window).on('hashchange', function() {
		// Split up into event type and params.
		var hash = window.location.hash.slice(1);
		var params = {};
		if(hash.indexOf('?') !== -1){
			var arr = hash.split('?');
			params = $.parseParams(arr[1]);
			hash = arr[0];
		}
		console.log(hash, params);

		// Handle event.
		if(hash === "watch"){
			var imdb_id = params.id;
			$('.loader').show();
			lookupItem(imdb_id).then((on) => {
				console.log(on);
				$('.loader').hide();
				currentItem = on;

				// Initialize frame.
				openPage({
					"path": "/static/quality.html",
					"allowFullScreen": false
				});
			});
		} else if(hash === "search"){
			$('#carousel_space').empty();
			$('#downloads').hide();
			$('.quota-bars').hide();
			// customRefreshTitle = "Executed search";
			// customRefreshMessage = "Successfully executed search on server.";
			// customRefreshTimer = 1000;
			setTimeout(() => {
				populateGrid((limit) => searchForItem(params.key), /*limit=*/12 * 1);
			}, 150);
		} else if(hash === "hide_recommendation"){
			var rec_obj = JSON.parse(params["obj"]);
			frontpage.hideRecommendation(rec_obj).then(() => {
				console.log("Successfully hid recommendation.");
				setTimeout(refreshHomepage, 150);
			});
		} else if(hash === "watched_recommendation"){
			$('.loader').show();
			var imdb_id = params["id"];
			addToHistory("movie", imdb_id).then(() => {
				refreshHistoryWatchlist().then(() => {
					refreshHistoryWatched().then(() => {
						$('.loader').hide();
						customRefreshTitle = "Added to history";
						customRefreshMessage = "Successfully marked item as watched.";
						customRefreshTimer = 1500;
						console.log("Successfully marked video as watched.");
						setTimeout(refreshHomepage, 150);
					});
				});
			});
		} else if(hash === "add_to_watchlist"){
			var imdb_id = params["id"];
			$('.loader').show();
			addToWatchlist("movie", imdb_id).then(() => {
				refreshHistoryWatchlist().then(() => {
					customRefreshTitle = "Added to watchlist";
					customRefreshMessage = "Successfully added item to watchlist.";
					customRefreshTimer = 1500;
					console.log("Successfully added video to watchlist.");
					$('.loader').hide();
					setTimeout(refreshHomepage, 150);
				});
			});
		} else if(hash === "refresh"){
			$('#downloads').hide();
			$('.quota-bars').hide();
			setTimeout(refreshHomepage, 10);
		} else if(hash === "reload"){
			setTimeout(() => {
				location.reload(true);
			}, 10);
		} else if(hash === "view_watchlist"){
			$('#downloads').hide();
			$('.quota-bars').hide();
			// customRefreshTitle = "Retrieved watchlist";
			// customRefreshMessage = "Successfully retrieved watchlist.";
			// customRefreshTimer = 1000;
			setTimeout(() => {
				populateGrid((limit) => {
					return new Promise((resolve, reject) => {
						getWatchlist().then((data) => {
							resolveParallel(data).then((resolved) => {
								resolve(resolved.resolved);
							});
						});
					});
				}, /*limit=*/12 * 1);
			}, 150);
		} else if(hash === "view_history"){
			$('#downloads').hide();
			$('.quota-bars').hide();
			// customRefreshTitle = "Retrieved history";
			// customRefreshMessage = "Successfully retrieved history.";
			// customRefreshTimer = 1000;
			setTimeout(() => {
				populateGrid((limit) => {
					return new Promise((resolve, reject) => {
						getWatched().then((data) => {
							resolveParallel(data).then((resolved) => {
								resolve(resolved.resolved);
							})
						})
					});
				}, /*limit=*/12 * 1);
			}, 150);
		} else if(hash === "view_downloads"){
			onHomepage = false;
			$('#carousel_space').empty();
			$('#grid').empty();
			$('.loader').show();
			var downloadInterval = null, populateDownloads = null;
			var populateDownloadsHelper = function() {
				getDownloads().then((downloads) => {
					var airplay_info = downloads.airplay_info;
					if(!downloads.downloads || !downloads.downloads.length){
						clearInterval(downloadInterval);
						downloadInterval = null;
						return;
					}
					var shouldRunAgain = populateDownloads(downloads.downloads, airplay_info);
					// console.log(shouldRunAgain);
					if(!shouldRunAgain || !$('#downloads').is(':visible')){
						clearInterval(downloadInterval);
						downloadInterval = null;
					}
				});
			};
			var toHHMMSS = function(val) {
			    var sec_num = parseInt(val, 10); // don't forget the second param
			    var hours   = Math.floor(sec_num / 3600);
			    var minutes = Math.floor((sec_num - (hours * 3600)) / 60);
			    var seconds = sec_num - (hours * 3600) - (minutes * 60);

			    if (hours   < 10) {hours   = "0"+hours;}
			    if (minutes < 10) {minutes = "0"+minutes;}
			    if (seconds < 10) {seconds = "0"+seconds;}
			    return hours+':'+minutes+':'+seconds;
			};
			var convertArrayOfObjectsToCSV = function(args) {
				// From https://halistechnology.com/2015/05/28/use-javascript-to-export-your-data-as-csv/
		        var result, ctr, keys, columnDelimiter, lineDelimiter, data;

		        data = args.data || null;
		        if (data == null || !data.length) {
		            return null;
		        }

		        columnDelimiter = args.columnDelimiter || ',';
		        lineDelimiter = args.lineDelimiter || '\n';

		        keys = Object.keys(data[0]);

		        result = '';
		        result += keys.join(columnDelimiter);
		        result += lineDelimiter;

		        data.forEach(function(item) {
		            ctr = 0;
		            keys.forEach(function(key) {
		                if (ctr > 0) result += columnDelimiter;

		                result += item[key];
		                ctr++;
		            });
		            result += lineDelimiter;
		        });

		        return result;
		    };
		    var downloadCSV = function(data, args) {
		    	// From https://halistechnology.com/2015/05/28/use-javascript-to-export-your-data-as-csv/
		        var data, filename, link;
		        var csv = convertArrayOfObjectsToCSV({
		            data: data
		        });
		        if (csv == null) return;

		        filename = args.filename || 'export.csv';

		        if (!csv.match(/^data:text\/csv/i)) {
		            csv = 'data:text/csv;charset=utf-8,' + csv;
		        }
		        data = encodeURI(csv);

		        link = document.createElement('a');
		        link.setAttribute('href', data);
		        link.setAttribute('download', filename);
		        link.click();
		    };
		    $('#libraryExportBtn').unbind("click");
		    $('#libraryExportBtn').click(function(e) {
		    	getDownloads().then((downloads) => {
		    		downloads = downloads.downloads;
		    		var data = [];
		    		for(var item of downloads){
		    			if(item.source === "oauth") continue;
		    			data.push({
		    				Filename: item.name,
		    				Title: (item.resolved ? item.resolved.title : "(unknown)"),
		    				Size: (item.size && item.size != -1 ? humanFileSize(item.size) : "(unknown)"),
		    				Collection: (item.collection && item.collection.length > 0 ? item.collection : "N/A")
		    			});
		    		}
		    		downloadCSV(data, {});
		    	});
		    });
			var progressMap = {};
			populateDownloads = (downloads, airplay_info) => {
				if(!airplay_info){
					return setTimeout(populateDownloadsHelper, 200);
				}
				if(airplay_info.currently_playing){
					$('#airplayProgress').show();
					var time_ratio = 100.0 * airplay_info.position / airplay_info.duration;
					var time_bar = $('.airplay-bar').find(".progress-bar");

					time_ratio = time_ratio.toFixed(2) + "%";
					time_bar.css("width", time_ratio);

					var time_desc = `${toHHMMSS(airplay_info.position)} of ${toHHMMSS(airplay_info.duration)} (${(100.0 * airplay_info.position / airplay_info.duration).toFixed(2)}% complete)`;
					time_bar.text(time_desc);
				} else {
					$('#airplayProgress').hide();
				}
				var tbody = $('#downloads').find("tbody");
				tbody.empty();
				downloads = downloads || [];
				// console.log("Downloads:", downloads);
				var keep_running = false;
				var icloud_count = 0, cloud_count = 0, in_progress_count = 0;
				for(var item of downloads) {
					var tr = $('<tr></tr>');
					var download_done = !item.progress;
					var title_inside = "<i>(not associated)</i>";
					var title_col_width = 75;
					if(item.resolved){
						title_col_width = 175;
						// title_inside = `<img src="${item.cover_image}"></img><caption>${item.title}</caption>`;
						title_inside = `<img src="${item.resolved.cover_image}" style="width: 100%;">`;
					}
					tr.append($('<td style="max-width:125px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">' + title_inside + '</td>'));
					var col_width = (download_done ? 175 : 75);
					var title_contents = (item.resolved ? `<b>${item.resolved.title}</b>` : "<i>(not associated)</i>");
					if(item.collection.length > 0){
						title_contents += `<br /><br /><b class="text-info">${item.collection}</b> collection`
					}
					tr.append($('<td style="max-width:' + col_width + 'px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">' + item.name + '</td>'));
					tr.append($('<td style="max-width:' + title_col_width + 'px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">' + title_contents + '</td>'));
					tr.append($('<td style="max-width:95px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">' + (item.size && item.size != -1 ? humanFileSize(item.size) : "N/A") + '</td>'));
					var inProgress = item.progress !== null && item.progress !== undefined && item.progress != 102.0 && (item.isDownloadingCloud || item.isDownloadingClient);
					if(!inProgress && item.isDownloadingCloud){
						inProgress = true;
						item.progress = "0.0";
					}
					var idDoneProgress = item.id in progressMap && progressMap[item.id];
					var imdbIdDoneProgress = item.resolved && item.resolved.imdb_code in progressMap && progressMap[item.resolved.imdb_code];
					if((idDoneProgress || imdbIdDoneProgress) && !inProgress){
						var desc = (item.hasDownloadedClient
									? `Successfully finished download of '${item.name}' in background.`
									: `Successfully finished cloud download of '${item.name}'.`);
						swal({
							title: "Download complete",
							text: desc,
							icon: "success",
							buttons: false,
							timer: 3000
						});
					}
					progressMap[item.id] = inProgress;
					if(item.resolved) progressMap[item.resolved.imdb_code] = inProgress;
					if(inProgress){
						keep_running = true;
						var prog = parseFloat(item.progress).toFixed(2);
						var desc = prog + "%";
						if(prog == 0.0){
							prog = 100.0;
							desc = "Collecting hosts...";
						} else if(prog == 101.0){
							prog = 100.0;
							desc = "Copying to folder...";
						}
						var prog_speed = '';
						if(item.progress_velocity){
							prog_speed = `<p style="margin-top: -10px; font-size: 1.0em;">${humanFileSize(item.progress_velocity)}/s ${item.avg_progress_velocity ? ("(" + humanFileSize(item.avg_progress_velocity) + "/s)") : ""}</p>`;
						}
						tr.append($('<td><div><div class="progress" style="margin-top: 10px;"><div class="progress-bar progress-bar-striped active" style="width: ' + prog + '%; min-width: 20%;">' + desc + '</div></div>' + prog_speed + '</div></td>'));
					} else {
						var watch_hash = '#watch_download?' + $.param({
							"folder_id": item.id,
							"imdb_id": item.resolved ? item.resolved.imdb_code : null
						});
						var watch_hash_icloud = '#watch_icloud?' + $.param({
							"icloud_id": item.id,
							"imdb_id": item.resolved ? item.resolved.imdb_code : null
						});
						var manage_collections = '#manage_collections?' + $.param({
							"icloud_id": item.id,
							"imdb_id": item.resolved ? item.resolved.imdb_code : null,
							"current_collection": item.collection
						});
						var buttons = [];
						if(item.hasUploadedClient){
							++icloud_count;
							buttons.push('<a href="' + watch_hash_icloud + '" class="btn btn-success">Stream from iCloud <span class="glyphicon glyphicon-film"></span></a>');
							buttons.push('<a href="' + manage_collections + '" class="btn btn-info">Manage Collections <span class="glyphicon glyphicon-list-alt"></span></a>');
						}
						if(item.hasDownloadedClient && item.isLocalToClient){
							if(item.hasUploadedClient){
								buttons.push('<a href="#" class="btn btn-danger btn-evict-local" role="button">Move to iCloud <span class="glyphicon glyphicon-cloud-upload"></span></a>');
							}
							buttons.push('<a href="#" class="btn btn-info btn-rename-local" role="button">Intelligent Rename <span class="glyphicon glyphicon-edit"></span></a>');
						}
						if(item.source === "oauth" && item.hasDownloadedCloud){
							++cloud_count;
							buttons.push('<a href="' + watch_hash + '" class="btn btn-success">Watch in Cloud <span class="glyphicon glyphicon-film"></span></a>');
							buttons.push('<a href="#" class="btn btn-primary btn-dl-background" role="button">Download in Background <span class="glyphicon glyphicon-download-alt"></span></a>');
							buttons.push('<a href="#" class="btn btn-danger btn-delete-cloud" role="button">Delete from Cloud <span class="glyphicon glyphicon-trash"></span></a>');
						}
						tr.append($(`<td>${buttons.join("<br /><br />")}</td>`));
						tr.find(".btn-evict-local").click(function(folder_id) {
							return function() {
								$('.loader').show();
								evictLocalItem(folder_id).then((data) => {
									$('.loader').hide();
									if(data.result !== true){
										swal({
											title: "Unable to move item to iCloud",
											text: data.err,
											icon: "error"
										});
									} else {
										swal({
											title: "Moved to iCloud",
											text: "Successfully moved item to iCloud.",
											icon: "success",
											buttons: false,
											timer: 3000
										}).then(() => {
											window.history.pushState(null, null, '#view_downloads');
											$(window).trigger('hashchange');
										});
									}
								});
							};
						}(item.id));
						tr.find(".btn-rename-local").click(function(item) {
							return function() {
								$('.loader').show();
								intelligentRenameItem({
									id: item.id,
									title: item.resolved ? item.resolved.title : "(unassociated)"
								}).then((data) => {
									$('.loader').hide();
									if(data.result !== true){
										swal({
											title: "Unable to rename item",
											text: data.err,
											icon: "error"
										});
									} else {
										swal({
											title: "Renamed item",
											text: `Successfully renamed item to '${data.new_name}'.`,
											icon: "success",
											buttons: false,
											timer: 3000
										}).then(() => {
											window.history.pushState(null, null, '#view_downloads');
											$(window).trigger('hashchange');
										});
									}
								});
							};
						}(item))
						tr.find(".btn-dl-background").click(function(folder_id) {
							return function() {
								$('.loader').show();
								retrieveFileUrl(folder_id, /*should_download=*/true).then((file_data) => {
									$('.loader').hide();
									console.log("fetch (dl):", file_data);
									var url = file_data.url;
									// window.location = url;
									// setTimeout(populateDownloadsHelper, 200);
									if(!file_data.name.endsWith(".mp4")){
										return swal({
											title: "Wrong file extension",
											text: `File extension is .${file_data.name.split(".").slice(-1)[0]}, not .mp4`,
											icon: "error"
										});
									}
									downloadItemInBackground({
										cloud_id: folder_id,
										uri: url,
										filename: file_data.name
									}).then((data) => {
										if(data.result !== true){
											swal({
												title: "Unable to download item",
												text: data.err,
												icon: "error"
											});
										} else {
											swal({
												title: "Download has begun",
												text: "Successfully began download in background.",
												icon: "success",
												buttons: false,
												timer: 3000
											}).then(() => {
												window.history.pushState(null, null, '#view_downloads');
												$(window).trigger('hashchange');
											});
										}
									});
								});
							};
						}(item.id));
						tr.find(".btn-delete-cloud").click(function(folder_id) {
							return function() {
								$('.loader').show();
								retrieveFileObj(folder_id).then((file_obj) => {
									console.log("file obj:", file_obj);
									apiReq("oauthQuery", {
										"function": "delete",
										"data": {
											"delete_arr": "[{\"type\": \"folder\", \"id\": \"" + file_obj.folder_id + "\"}]"
										}
									}, function(res) {
										console.log("del output:", res);
										$('.loader').hide();
										swal({
											title: "Deleted item",
											text: "Successfully deleted item.",
											icon: "success",
											buttons: false,
											timer: 3000
										});
										setTimeout(populateDownloadsHelper, 200);
									});
								});
							};
						}(item.id));
					}
					if(!item.imdb_id || item.imdb_id.length == 0){
						tr.addClass("danger");
					} else if(inProgress || item.isUploadingClient){
						tr.addClass("active");
						tr.find(".progress-bar").addClass("progress-bar-info");
						++in_progress_count;
					} else if(item.isLocalToClient){
						tr.addClass("warning");
					} else if(item.resolved && isMovieInWatched(item.resolved.imdb_code)) {
						tr.addClass("success");
					}
					var currentStatus = "";
					if(item.isDownloadingCloud) currentStatus = "Cloud is downloading";
					else if(item.isDownloadingClient) currentStatus = "Client is downloading";
					else if(item.isUploadingClient) currentStatus = "Client is uploading";
					else if(item.hasUploadedClient) currentStatus = "In iCloud Drive";
					else if(item.hasDownloadedClient) currentStatus = "Only in Disk"
					else if(item.hasDownloadedCloud) currentStatus = "Only in Cloud";
					tr.append(`<td class="h5">${currentStatus}</td>`);
					tr.find("td").css('vertical-align', 'middle');
					tbody.append(tr);
				}
				$('#libraryDesc').text(`${downloads.length} item${downloads.length == 1 ? "" : "s"} in library (${icloud_count} in iCloud, ${cloud_count} in Cloud, ${in_progress_count} in progress)`);
				apiReq("oauthQuery", {
					"function": "get_memory_bandwidth",
					"data": {}
				}, function(data) {
					for(var k in data){
						data[k] = parseFloat(data[k]);
					}
					var space_ratio = 100.0 * data.space_used / data.space_max;
					var bandwidth_ratio = 100.0 * data.bandwidth_used / data.bandwidth_max;
					var space_bar = $('.space-bar').find(".progress-bar");
					var bandwidth_bar = $('.bandwidth-bar').find(".progress-bar");

					space_ratio = space_ratio.toFixed(2) + "%";
					bandwidth_ratio = Math.min(bandwidth_ratio, 100.0).toFixed(2) + "%";
					space_bar.css("width", space_ratio);
					bandwidth_bar.css("width", bandwidth_ratio);

					var space_desc = humanFileSize(data.space_used) + "/" + humanFileSize(data.space_max);
					var bandwidth_desc = humanFileSize(data.bandwidth_used) + "/" + humanFileSize(data.bandwidth_max);
					space_bar.text(space_desc);
					bandwidth_bar.text(bandwidth_desc);
				});
				return keep_running;
			};
			getDownloads().then((downloads) => {
				var shouldRunAgain = populateDownloads(downloads.downloads, downloads.airplay_info);
				if(shouldRunAgain){
					downloadInterval = setInterval(populateDownloadsHelper, 4000);
				}
				$('.loader').hide();
				$('#downloads').show();
				$('.quota-bars').show();
			});
		} else if(hash === "watch_download"){
			var folder_id = params.folder_id;
			resolveItem(params.imdb_id).then((on) => {
				retrieveFileUrl(folder_id, /*should_download=*/false).then((file_data) => {
					console.log("fetch:", file_data);
					var url = file_data.url;
					lastDownloadedItem = {
						url: url,
						item: on
					};
					openPage({
						"path": "/static/watch.html",
						"allowFullScreen": true
					});
				});
			});
		} else if(hash === "watch_icloud"){
			var cloud_id = params.icloud_id;
			$('.loader').show();
			resolveItem(params.imdb_id).then((on) => {
				iCloudStreamUrl(cloud_id).then((file_data) => {
					$('.loader').hide();
					console.log("fetch:", file_data);
					var url = file_data.url;
					lastDownloadedItem = {
						url: url,
						item: on
					};
					openPage({
						"path": "/static/watch.html",
						"allowFullScreen": true
					});
				});
			});
		} else if(hash === "associate_imdb"){
			$('.loader').show();
			resolveItem(params.id).then((on) => {
				getDownloads().then((downloads) => {
					console.log(on);
					$('.loader').hide();
					currentItem = {
						item: on,
						downloads: downloads.downloads
					};

					// Initialize frame.
					openPage({
						"path": "/static/quality.html#downloads",
						"allowFullScreen": false
					});
				});
			});
		} else if(hash === "manage_collections"){
			if(!params.imdb_id || !params.imdb_id.length){
				swal({
					icon: "error",
					title: "Not associated",
					text: "Item needs to be associated first"
				});
			} else {
				$('.loader').show();
				resolveItem(params.imdb_id).then((on) => {
					getCollections().then((collections) => {
						console.log(on);
						$('.loader').hide();
						currentItem = {
							item: on,
							collections: collections,
							icloud_id: params.icloud_id,
							current_collection: params.current_collection
						};

						// Initialize frame.
						openPage({
							"path": "/static/quality.html#collections",
							"allowFullScreen": false
						});
					});
				});
			}
		}

		// Mark done by resetting window.location.hash
		window.history.pushState(null, null, '#');
	});

	// Listen for events from modals.
	window.addEventListener("message", (e) => {
		var parsed = JSON.parse(e.data);
		var type = parsed.type;
		var data = parsed.data;
		console.log("Received message from frame of type:", type, "with data:", data);
		if(type === "quality_window_open"){
			sendFrameMessage(currentItem);
		} else if(type === "quality_select"){
			$('#frameModal').modal('hide');
			$('.loader').show();
			fetchItem(data).then((data) => {
				console.log("fetch result:", data);
				$('.loader').hide();
				if(data.result !== true){
					if(!data.enqueued){
						swal({
							title: "Unable to download item",
							text: (data.not_enough_space ? "Not enough space available in cloud" : data.result),
							icon: "error"
						});
					} else {
						swal({
							title: "Item queued",
							text: "Added to queue",
							icon: "success",
							buttons: false,
							timer: 3000
						});
					}
				} else {
					swal({
						title: "Download has begun",
						text: "Successfully began download.",
						icon: "success",
						buttons: false,
						timer: 3000
					}).then(() => {
						window.history.pushState(null, null, '#view_downloads');
						$(window).trigger('hashchange');
					});
				}
			});
		} else if(type === "associate_select") {
			$('#frameModal').modal('hide');
			$('.loader').show();
			associateItem(data).then((data) => {
				console.log("associate result:", data);
				$('.loader').hide();
				if(data.result !== true){
					swal({
						title: "Unable to associate item",
						icon: "error"
					});
				} else {
					swal({
						title: "Item successfully associated",
						text: "Successfully associated item.",
						icon: "success",
						buttons: false,
						timer: 2000
					}).then(() => {
						window.history.pushState(null, null, '#view_downloads');
						$(window).trigger('hashchange');
					});
				}
			});
		} else if(type === "watch_window_open"){
			$('.loader').show();
			getScrobbles().then((scrobbles) => {
				if(lastDownloadedItem.item){
					for(var item of scrobbles){
						if(item && item.movie && item.movie.ids && item.movie.ids.imdb === lastDownloadedItem.item.imdb_code){
							lastDownloadedItem.item.playback_progress = item.progress;
						}
					}
				}
				$('.loader').hide();
				sendFrameMessage(lastDownloadedItem);
			});
		} else if(type === "associate_window_open"){
			sendFrameMessage(currentItem);
		} else if(type === "collections_window_open"){
			sendFrameMessage(currentItem);
		} else if(type === "update_watch_status"){
			updateWatchStatus(data.imdb_code, data.progress, data.state).then(function(data) {
				return function(ret) {
					console.log("Set state to '%s' and progress to '%.3f' for imdb code '%s'", data.state, data.progress, data.imdb_code);
				};
			}(data));
		} else if(type === "start_airplay"){
			closePage();
			startAirplay(data).then((info) => {
				if(!info || !info.result){
					swal({
						title: "Unable to start playback",
						icon: "error"
					});
				} else {
					swal({
						title: "AirPlay playback started",
						text: "Successfully started playback.",
						icon: "success",
						buttons: false,
						timer: 2000
					}).then(() => {
						window.history.pushState(null, null, '#view_downloads');
						$(window).trigger('hashchange');
					});
				}
			});
		} else if(type === "search_tv_episode"){
			// .imdb_code, .episode, .season
			// ... TODO
		} else if(type === "add_to_collection") {
			$('#frameModal').modal('hide');
			$('.loader').show();
			addToCollection(data).then((data) => {
				console.log("associate result:", data);
				$('.loader').hide();
				if(data.result !== true){
					swal({
						title: "Unable to add to collection",
						text: data.err,
						icon: "error"
					});
				} else {
					swal({
						title: "Item successfully added",
						text: "Successfully added item to collection.",
						icon: "success",
						buttons: false,
						timer: 2000
					}).then(() => {
						window.history.pushState(null, null, '#view_downloads');
						$(window).trigger('hashchange');
					});
				}
			});
		}
	}, false);

	// Handle search form submission.
	$('#search-form').submit((e) => {
		e.preventDefault();
		window.location.hash = 'search?key=' + encodeURIComponent($('#search-input').val());
	});
});





























