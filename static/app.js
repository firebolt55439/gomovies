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
        // se si chiude la modale resettiamo i dati dell'iframe per impedire ad un video di continuare a riprodursi anche quando la modale è chiusa
        this.on('hidden.bs.modal', function(){
          $(this).find('iframe').html("").attr("src", "");
        });
      
        return this;
    };
})();

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

function fetchItem(uri) {
	return new Promise((resolve, reject) => {
		apiReq("fetchUri", {
			"uri": uri
		}, function(data) {
			resolve(data);
		});
	});
}

// Run on page load.
let player_windows = [];
let win_id = 0;
let child, currentItem
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
	
	// Function to generate markup for movie poster item in grid.
	var retrieveCoverMarkup = function(on) {
		var li = $('<li></li>');
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
		var anc = $('<a class="grid-cell" href="' + new_hash + '"></a>');
		anc.append('<img class="grid-img" src="' + on.cover_image + '" />');
		anc.append('<span class="grid-overlay"></span>');
		var desc = [on.mpaa_rating || "NR"];
		var sort_order = {
			"3D": 1,
			"HD": 2,
			"1080p": 3,
			"720p": 4
		}
		var qualities = on.sources.map((x) => x.quality).sort((a, b) => {
			var x = sort_order[a], y = sort_order[b];
			return (y - x);
		});
		qualities = qualities.filter((x, i) => qualities.indexOf(x) === i); // remove duplicates
		desc = on.title + " <br /> " + desc.concat(qualities).join(" | ");
		var grid_span = $('<span class="grid-text"></span>');
		grid_span.html(desc);
		anc.append(grid_span);
		if(on.recommendation){
			var hide_hash = "#hide_recommendation?" + $.param({
				"obj": JSON.stringify(on.recommendation)
			});
			anc.append($('<span class="grid-button-right"><a href="' + hide_hash + '" class="btn btn-danger"><span class="glyphicon glyphicon-remove"></span></a></span>'));
		}
		if(false && history.isMovieInWatchlist(on.imdb_code)){
			anc.append($('<a class="top-left-corner btn btn-info"><span class="glyphicon glyphicon-th-list"></span></a>'));
		}
		if(false && history.hasFinishedMovie(on.imdb_code)){
			anc.append($('<a class="top-right-corner btn btn-success"><span class="glyphicon glyphicon-check"></span></a>'));
			anc.append($('<span class="grid-button-bottom-left"><a href="' + watched_hash + '" class="btn btn-info"><span class="glyphicon glyphicon-check"></span></a></span>'));
		} else {
			anc.append($('<span class="grid-button-bottom-left"><a href="' + watched_hash + '" class="btn btn-success"><span class="glyphicon glyphicon-check"></span></a></span>'));
			anc.append($('<span class="grid-button-bottom-right"><a href="' + watchlist_hash + '" class="btn btn-primary"><span class="glyphicon glyphicon-th-list"></span></a></span>'));
		}
		li.append(anc);
		return li;
	};
	var retrieveBlankCoverMarkup = function() {
		var li = $('<li></li>');
		var anc = $('<a class="grid-cell" href="#"></a>');
		anc.append('<img class="grid-img" />');
		anc.append('<span class="grid-overlay"></span>');
		anc.append('<span class="grid-text">(Divider)</span>');
		li.append(anc);
		return li;
	};
	/*
	apiReq("resolveParallel", {
		"ids": ["tt0108052", "tt1408101", "tt0075860", "tt0120815", "tt0264464"]
	}, console.log);
	*/
	
	// Populate grid with top movies by default, or requested movies if search term exists.
	var onHomepage = false, autoPopulationCounter = 0;
	var populateGrid = (callback, limit) => {
		onHomepage = (callback === getRecommendedMovies); // for auto-population
		autoPopulationCounter = 0; // reset auto-population counter
		$('.loader').show();
		callback(limit).then((data) => {
			console.log(data);
			
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
				while(true){
					if(highlighted.length == data.length) break;
					ind = Math.min(Math.round(Math.random() * data.length), data.length - 1);
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
				img_div.append($('<img src="' + cur.cover_image + '" alt="' + cur.title + '" />'));
				var cap = $('<div class="carousel-caption"></div>');
				cap.append($('<div class="carousel-title"><h3 style="font-size: 2vw;">' + cur.title + ' [' + (cur.mpaa_rating || "NR") + ']</h3></div>'));
				cap.append($(retrieveRatingMarkup(cur.imdb_rating * 10.0)));
				var summary = cur.summary;
				if(!summary || !summary.length) summary = "(no description available)";
				cap.append($('<p class="carousel-summary">' + summary + '</p>'));
				var new_hash = '#watch?' + $.param({
					"title": cur.title,
					"id": cur.imdb_code
				});
				cap.append($('<a href="' + new_hash + '" class="btn btn-success">Watch <span class="glyphicon glyphicon-film"></span></a><br /><br />'));
				// TODO: Add star IMDB rating as well
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
			$('.loader').hide();
		});
	};
	var refreshHomepage = function() {
		populateGrid(getRecommendedMovies, /*limit=*/12 * 1);
	};
	refreshHomepage();
	
	// Detect when user has hit bottom of scrollable view and populate with new movies.
	document.addEventListener('scroll', function (event) {
		if(document.body.scrollHeight == document.body.scrollTop + window.innerHeight) {
			if(onHomepage){
				++autoPopulationCounter;
				console.log("Hit rock bottom.", autoPopulationCounter);
				$('.loader').show();
				disableScroll();
				getRecommendedMovies(12, autoPopulationCounter).then((data) => {
					console.log(data);
					
					// Inform user.
					let notif = new Notification('Extended Results', {
						body: "Added " + data.length + " result(s).",
						silent: true
					});
					notif.onclick = () => {};
					
					// Append to grid.
					$('#grid').append(retrieveBlankCoverMarkup());
					for(var on of data){
						var li = retrieveCoverMarkup(on);
						$('#grid').append(li);
					}
					$('.loader').hide();
					enableScroll();
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
	var sendFrameMessage = function(obj) {
		var contentWin = $('#frameModal').find("iframe").get(0).contentWindow;
		contentWin.postMessage(JSON.stringify(currentItem), "*");
	}

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
				
				// Fill in playback and/or history information for current item.
				currentItem.playback_progress = undefined/*history.getPlaybackProgressForMovie(imdb_id)*/; // TODO
				
				// Initialize frame.
				openPage({
					"path": "/static/quality.html",
					"allowFullScreen": false
				});
			});
		} else if(hash === "search"){
			$('#carousel_space').empty();
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
			var imdb_id = params["id"];
			frontpage.markWatchedById(imdb_id).then((status) => {
				console.log("Successfully marked video as watched.");
				console.log(status);
				setTimeout(refreshHomepage, 150);
			});
		} else if(hash === "add_to_watchlist"){
			var imdb_id = params["id"];
			$('.loader').show();
			frontpage.addToWatchlist(imdb_id).then(() => {
				console.log("Successfully added video to watchlist.");
				$('.loader').hide();
				
				// Inform user.
				var item = metadata.getItemById(imdb_id);
				let notif = new Notification('Added To Watchlist', {
					body: ("Added " + item.title + " to watchlist."),
					icon: item.cover_image,
					silent: true
				});
			});
		} else if(hash === "refresh"){
			setTimeout(refreshHomepage, 10);
		} else if(hash === "reload"){
			setTimeout(() => {
				location.reload(true);
			}, 10);
		} else if(hash === "view_watchlist"){
			setTimeout(() => {
				populateGrid(frontpage.getWatchlist, /*limit=*/12 * 1);
			}, 150);
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
			});
			// ...
		}
	}, false);
	
	// Listen for quality selection.
	/*
	ipc.on('quality_select', (event, msg) => {
		// Start loading indicator
		$('.loader').show();
		
		// Close modal window
		var chosen_url = msg;
		child.close();
		++win_id;
	
		// Select appropriate url
		var download_uri = null;
		var selected = currentItem.sources.filter((x) => x.url === chosen_url)[0];
		download_uri = selected.url;
		console.log(selected);
	
		// Pass off to video player
		if(download_uri === null) console.error("Could not select download URI!");
		let win = new electron.remote.BrowserWindow({
			title: currentItem.title,
			modal: false,
			show: false
		})
	
		win.loadURL(url.format({
			pathname: path.join(__dirname, 'video.html'),
			protocol: 'file:',
			slashes: true
		}))

		// Send video info when page is ready
		win.webContents.on('did-finish-load', () => {
			win.webContents.send('video', [currentItem, download_uri, win_id]);
			//win.show(); //
			player_windows.push(win);
		});
		
		ipc.on('max-' + win_id, (evt, msg) => {
			console.log("Received window max request.");
			//win.maximize();
			//win.setFullScreen(true);
		});
		
		ipc.on('started-' + win_id, (evt, msg) => {
			$('.loader').hide();
		});

		// Open the DevTools if needed.
		//win.webContents.openDevTools(); win.show();

		// Emitted when the window is closed.
		win.on('close', function () {
			// Free window object
			player_windows.splice(player_windows.indexOf(win), 1);
			win = null;
		});
	});
	*/
	
	// Handle IPC data events.
	/*
	ipc.on('tv_show_info', async function(event, imdb_code) {
		var video_obj = await metadata.getTraktItemById(imdb_code);
		console.log("TV show event:", event);
		video_obj = video_obj.show;
		console.log(video_obj);
		if(video_obj){
			video_obj = await trakt.seasons.summary({
				id: imdb_code,
				extended: "full"
			});
		}
		console.log("seasons:", video_obj);
		ipc.send('reply', {
			type: "tv_show_info",
			data: video_obj
		});
	});
	ipc.on('search_tv_episode', async function(event, data) {
		var imdb_code = data.imdb_code;
		var season = data.season, episode = data.episode;
		var item = metadata.getItemById(imdb_code);
		var results = await search.findSourcesByItem(item, {
			season: season,
			episode: episode
		});
		console.log("Found TV episode sources:", results);
		ipc.send('reply', {
			type: "search_tv_episode",
			data: results
		});
	});
	*/
	
	// Handle search form submission.
	$('#search-form').submit((e) => {
		e.preventDefault();
		window.location.hash = 'search?key=' + encodeURIComponent($('#search-input').val());
	});
});





























