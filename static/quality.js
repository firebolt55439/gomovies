function commaSeparateNumber(val){
	while (/(\d+)(\d{3})/.test(val.toString())){
		val = val.toString().replace(/(\d+)(\d{3})/, '$1'+','+'$2');
	}
	return val;
}

function renderOptions(event, item){
    //console.log("modal:", sources);
    // Fill in information fields.
    $('#cover-img').attr('src', item.cover_image);
    $('#summary-text').text(item.summary);
    $('#mpaa-rating').text(item.mpaa_rating);
    $('.imdb-rating').css('width', (item.imdb_rating * 10) + "%");
    $('.imdb-rating').html(item.imdb_rating + "/10 IMDb");
    $('#imdb-rating-count').html(commaSeparateNumber(item.imdb_rating_count));
    $('#video-title').html(item.title);
    
    // Fill in source selection.
    var fillInOptions = function(sources) {
    	$('#options').empty();
		var sort_order = {
			"4K": 1,
			"3D": 2,
			"HD": 3,
			"1080p": 4,
			"720p": 5,
			"SD": 6
		}
		var by_quality = Object.keys(sort_order).map(x => []);
		for(var on of sources){
			var idx = sort_order[on.quality] - 1;
			by_quality[idx].push(on);
		}
		by_quality = by_quality.map((arr) => {
			arr.sort((a, b) => {
				return b.sources - a.sources;
			});
			return arr;
		});
		sources = [];
		for(var on of by_quality) sources = sources.concat(on);
		var getScaledhosts;
		if(sources.length > 0){
			var host_counts = sources.map((x) => parseInt(x.sources));
			var max_hosters = host_counts.reduce((a, b) => Math.max(a, b));
			var min_hosters = host_counts.reduce((a, b) => Math.min(a, b));
			// (min, 0) --> (max, 100); m = 100.0 / (max - min);
			var slope = 100.0 / (max_hosters - min_hosters);
			var y_intercept = 100.0 - (slope * max_hosters);
			if(max_hosters != min_hosters){
				getScaledhosts = (hosts) => (slope * hosts + y_intercept);
			} else {
				getScaledhosts = (hosts) => Math.max(Math.min(hosts, 100), 0);
			}
		} else {
			$('#quality_header').hide();
			$('#none_available').show();
		}
		for(var on of sources){
			var li = $('<li class="opt-select" data-url="' + on.url + '"></li>');
			if(on.filename){
				li.attr("data-toggle", "tooltip");
				li.attr("title", on.filename);
			}
			var desc_arr = [on.quality, on.size, on.sources + " hosts", on.clients + " clients", on.source];
			if(on.tv) desc_arr.unshift("S" + on.tv.season + "E" + on.tv.episode);
			li.text(desc_arr.join(" | "));
			var scaled = getScaledhosts(parseInt(on.sources));
			var rgb_components = [(255 * (100.0 - scaled)) / 100, ((255 * scaled) / 100.0), 0.0]
			rgb_components = rgb_components.map((x) => Math.round(x).toString());
			var scaled_rgb = "rgb(" + rgb_components.join(",") + ")";
			//li.css('background-color', scaled_rgb);
			//li[0].style.backgroundColor = scaled_rgb;
			if(sources.length === 1){
				scaled_rgb = "rgb(128,128,0)";
			}
			li.css('background-color', scaled_rgb);
			//console.log(parseInt(on.sources), scaled, scaled_rgb, li);
			$('#options').append(li);
		}
		
		// Install click handler.
		$('.opt-select').click(function() {
			var that = $(this);
			var url = that.data("url");
			//console.log("selected url:", url);
			swal({
				title: "Choose Method",
				text: "How would you like to download this?",
				buttons: {
					cancel: "Cancel",
					myself: {
						text: "Myself",
						value: "myself"
					},
					cloud: {
						text: "Via Cloud",
						value: "cloud"
					}
				},
				icon: item.cover_image
			}).then((value) => {
				if(value === "cloud"){
					window.parent.postMessage(JSON.stringify({
						type: "quality_select",
						data: url
					}), "*");
					throw null;
				} else if(value === "myself"){
					return swal({
						title: "Are you sure?",
						text: "Would you really like to download this yourself?",
						icon: "warning",
						buttons: {
							cancel: true,
							confirm: {
								text: "Yes, I am sure"
							}
						},
						dangerMode: true
					});
				}
			}).then(value => {
				if(value){
					swal({
						title: "Download has begun",
						text: "Successfully began download.",
						icon: "success",
						buttons: false,
						timer: 3000
					});
					window.location.href = url;
				}
			}).catch(err => {
				if(!err){
					swal.stopLoading();
					swal.close();
				} else {
					swal("Error!", err.toString(), "error");
				}
			});
		});
	
		// Initialize tooltips.
		$('[data-toggle="tooltip"]').tooltip();
    };
    fillInOptions(item.sources);
    
    // Handle TV shows.
    /*
    var main_console = remote.getGlobal("console");
    //main_console.log(item);
    if(item.is_tv_show){
    	$('.tv-container').removeClass("hidden");
    	ipc.send('forward-to-render', {
			type: "tv_show_info",
			data: item.imdb_code
		});
		ipc.on('tv_show_info', (evt, tv_show_info) => {
			console.log("Got tv show info:", tv_show_info);
			var seasons = [];
			var episodesBySeason = {};
			for(var on of tv_show_info){
				if(on.number > 0){
					seasons.push([on.title, on.number]);
					episodesBySeason[on.number] = [on.aired_episodes, on.episode_count];
				}
			}
			for(var on of seasons){
				var season_title = on[0], season_num = on[1];
				$('#seasonsel').append($('<option value="' + season_num + '">' + season_title + '</option>'));
			}
			$('#seasonsel').change(() => {
				var season_num = parseInt($('#seasonsel').val());
				$('#episodesel').empty();
				var epcount = episodesBySeason[season_num];
				epcount[0] = Math.min(epcount[0] + 1, epcount[1]); // hacky; in case the latest episode *has* actually aired
				for(var i = 0; i < epcount[1]; i++){
					var desc = "Episode " + (i + 1);
					if(i >= epcount[0]) desc += " (not aired yet)";
					$('#episodesel').append($('<option value="' + (i+1) + '">' + desc + '</option>'));
				}
			});
			$('#seasonsel').val(1).trigger("change");
		});
		$('#tv_search_btn').click(() => {
			$('#tv_search_btn').blur();
			$('.loader').show();
			ipc.send('forward-to-render', {
				type: "search_tv_episode",
				data: {
					imdb_code: item.imdb_code,
					season: $('#seasonsel').val(),
					episode: $('#episodesel').val()
				}
			});
		});
		ipc.on("search_tv_episode", (evt, search_results) => {
			main_console.log("got tv show search results:", search_results);
			fillInOptions(search_results);
			$('.loader').hide();
		});
    }
    */
}

$(function() {
	//console.log("init'd quality");
	window.addEventListener("message", (e) => {
		setTimeout(() => {
			console.log("Got a message:", e);
			renderOptions(e, JSON.parse(e.data));
		}, 10);
	}, false);
	
	window.parent.postMessage(JSON.stringify({
		"type": "quality_window_open",
		"data": {}
	}), "*");
});