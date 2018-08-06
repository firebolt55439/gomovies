function commaSeparateNumber(val){
	while (/(\d+)(\d{3})/.test(val.toString())){
		val = val.toString().replace(/(\d+)(\d{3})/, '$1'+','+'$2');
	}
	return val;
}

var fillInOptions;

function renderOptions(event, item){
    //console.log("modal:", sources);
    // Fill in information fields.
    var downloads, collections, current_collection, icloud_id;
    var isAssociate = false, isCollections = false;
    if(item.downloads){
    	downloads = item.downloads;
    	item = item.item;
    	isAssociate = true;
    	$('#quality_header').text("Select Download:");
    	$('#none_available').text("No downloads exist.");
    } else if(item.collections){
    	current_collection = item.current_collection;
    	icloud_id = item.icloud_id;
    	collections = item.collections;
    	item = item.item;
    	isCollections = true;
    	$('#quality_header').text("Select Collection:");
    	$('#none_available').text("No collections exist.");
    } else {
    	$('#quality_header').text("Select Source:");
    	$('#none_available').text("No sources available.");
    }
    $('#cover-img').attr('src', item.cover_image);
    $('#summary-text').text(item.summary);
    $('#mpaa-rating').text(item.mpaa_rating);
    $('.imdb-rating').css('width', (item.imdb_rating * 10) + "%");
    $('.imdb-rating').html(item.imdb_rating + "/10 IMDb");
    $('#imdb-rating-count').html(commaSeparateNumber(item.imdb_rating_count));
    $('#video-title').text(item.title);
    $('#runtime').text(item.runtime);
    $('#genres').text(item.genres.join(", "));
    if(Number.isInteger(item.rotten_tomatoes)){
    	$('.rt-rating').css('width', item.rotten_tomatoes + "%");
    	$('.rt-rating').html(item.rotten_tomatoes + "% RT");
    } else {
    	$('.rt-rating').parent().hide();
    }
    if(Number.isInteger(item.metacritic)){
	    $('.metacritic-rating').css('width', item.metacritic + "%");
	    $('.metacritic-rating').html(item.metacritic + " Metascore");
	} else {
		$('.metacritic-rating').parent().hide();
	}
	if(item.awards.length > 0){
		$('#awards').text(item.awards);
	} else {
		$('#awards_p').hide();
	}
	if(item.cast.length > 0){
		$('#cast').text(item.cast);
	} else {
		$('#cast').hide();
	}

    // Fill in source selection.
    fillInOptions = function(sources) {
    	$('#options').empty();
    	if(isAssociate){
    		var pretty_source = {
    			"oauth": "Cloud",
    			"disk": "iCloud Drive"
    		};

    		// Generate list
    		console.log(downloads);
    		var sortFn = (a, b) => {
    			if(a.name < b.name) return -1;
    			if(a.name > b.name) return 1;
    			return 0;
    		};
    		downloads = downloads.filter(x => x.imdb_id.length == 0).sort(sortFn).concat(downloads.filter(x => x.imdb_id.length > 0));
    		for(var on of downloads){
    			var li = $('<li class="opt-select" data-name="' + on.name + '" data-id="' + on.id + '"></li>');
    			var desc_arr = [];
    			desc_arr.push(on.name);
    			desc_arr.push(pretty_source[on.source]);
    			li.text(desc_arr.join(" | "));
    			if(on.imdb_id.length == 0){
    				li.css('background-color', "rgb(0,128,0)");
    			} else {
    				li.css('background-color', "rgb(0,0,128)");
    			}
    			$('#options').append(li);
    		}
    		if(downloads.length == 0){
    			$('#quality_header').hide();
    			$('#none_available').show();
    		}

    		// Handle clicks.
    		$('.opt-select').click(function() {
    			var that = $(this);
    			var name = that.data("name");
    			var id = that.data("id");
    			swal({
    				title: "Are you sure?",
    				icon: "warning",
    				text: `Is '${name}' this downloaded item?`,
    				buttons: {
    					cancel: true,
    					confirm: {
    						text: "Yes, it is!"
    					}
    				},
    				icon: item.cover_image
    			}).then((value) => {
    				if(!value) return;
    				console.log("Association confirmed.");
    				window.parent.postMessage(JSON.stringify({
    					type: "associate_select",
    					data: {
    						cloud_id: id,
    						imdb_code: item.imdb_code
    					}
    				}), "*");
    			});
    		});
    		return;
    	}
    	if(isCollections){
    		collections.push({
    			name: "",
    			count: null
    		});
    		for(var on of collections){
    			var li = $('<li class="opt-select" data-name="' + on.name + '"></li>');
    			if(on.count != null){
    				var desc_arr = [];
    				desc_arr.push(on.name);
    				desc_arr.push(`${on.count} item${on.count == 1 ? "" : "s"}`);
    				li.text(desc_arr.join(" | "));
    				if(current_collection === on.name){
    					li.css('background-color', "rgb(0,128,0)");
    					li.addClass("disabled");
    				} else {
    					li.css('background-color', "rgb(0,0,128)");
    				}
    			} else {
    				li.text("(make a new collection)");
    				li.addClass("new_collection");
    				li.css('background-color', "rgb(0,128,128)");
    			}
    			$('#options').append(li);
    		}
    		if(collections.length == 0){
    			$('#quality_header').hide();
    			$('#none_available').show();
    		}
    		// Handle clicks.
    		var collectionConfirmHandler = function(name) {
    			swal({
    				title: "Add to collection?",
    				icon: "warning",
    				text: `Do you want to add this to the '${name}' collection?`,
    				buttons: {
    					cancel: true,
    					confirm: {
    						text: "Yes, I do!"
    					}
    				},
    				icon: item.cover_image
    			}).then((value) => {
    				if(!value) return;
    				console.log("Adding to collection confirmed.");
    				window.parent.postMessage(JSON.stringify({
    					type: "add_to_collection",
    					data: {
    						cloud_id: icloud_id,
    						collection_id: name
    					}
    				}), "*");
    			});
    		};
    		$('.opt-select').click(function() {
    			var that = $(this);
    			var name = that.data("name");
    			if(that.hasClass("disabled")){
    				return swal({
    					title: "Already in collection",
    					text: `This already belongs to the '${name}' collection.`,
    					icon: "error",
    					timer: 3000,
    					buttons: false
    				});
    			}
    			if(that.hasClass("new_collection")){
    				// ...
    				swal({
    					title: "Name of collection?",
    					text: "Please enter the name of the new collection",
    					content: "input",
    					buttons: {
    						cancel: true,
    						confirm: {
    							text: "Create collection!"
    						}
    					}
    				}).then((value) => {
    					if(!value) return;
    					collectionConfirmHandler(value);
    				});
    				return;
    			}
    			collectionConfirmHandler(name);
    		});
    		return;
    	}
		var sort_order = {
			"4K": 1,
			"3D": 2,
			"2160p": 3,
			"HD": 4,
			"1080p": 5,
			"720p": 6,
			"SD": 7
		};
		var by_quality = Object.keys(sort_order).map(x => []);
		for(var on of sources){
			var idx = sort_order[on.quality] - 1;
			if(idx === undefined || (!idx && idx != 0)) idx = sort_order["SD"] - 1;
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
			if(on.tv){
				if(on.tv.episode < 10000) desc_arr.unshift("S" + on.tv.season + "E" + on.tv.episode);
				else desc_arr.unshift("S" + on.tv.season);
			}
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
						data: {
							uri: url,
							imdb_code: item.imdb_code
						}
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
    if(item.is_tv_show){
    	$('.tv-container').removeClass("hidden");
    	$('#seasonsel').val("1");
    	$('#episodesel').val("1");
    	$('#tv_search_btn').click(() => {
    		$('#tv_search_btn').blur();
    		$('.loader').show();
    		window.parent.postMessage(JSON.stringify({
    			type: "search_tv_episode",
    			data: {
    				imdb_code: item.imdb_code,
    				season: parseInt($('#seasonsel').val(), 10),
    				episode: parseInt($('#episodesel').val(), 10)
    			}
    		}), "*");
    	});
    }
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

	var win_type;
	if(!window.location.hash){
		win_type = "quality";
	} else if(window.location.hash === "#downloads"){
		win_type = "associate";
	} else if(window.location.hash === "#collections"){
		win_type = "collections";
	}
	window.parent.postMessage(JSON.stringify({
		"type": `${win_type}_window_open`,
		"data": {}
	}), "*");
});
